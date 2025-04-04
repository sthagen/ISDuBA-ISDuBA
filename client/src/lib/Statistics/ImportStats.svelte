<!--
 This file is Free Software under the Apache-2.0 License
 without warranty, see README.md and LICENSES/Apache-2.0.txt for details.

 SPDX-License-Identifier: Apache-2.0

 SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
 Software-Engineering: 2024 Intevation GmbH <https://intevation.de>
-->

<script lang="ts">
  import type { ErrorDetails } from "$lib/Errors/error";
  import ErrorMessage from "$lib/Errors/ErrorMessage.svelte";
  import {
    fetchImportFailuresStatistic,
    type StatisticGroup,
    type StatisticType,
    fetchBasicStatistic,
    getCVSSTextualRating,
    type CVSSTextualRating,
    mergeImportFailureStatistics,
    fetchTotals,
    getCVSSTextualRatingDescription,
    getLabelForKey
  } from "$lib/Statistics/statistics";
  import Chart from "chart.js/auto";
  import { Button, ButtonGroup, Spinner } from "flowbite-svelte";
  import { onDestroy, onMount } from "svelte";
  import "chartjs-adapter-moment";
  import StatsTable from "./StatsTable.svelte";
  import {
    HOUR_MS,
    YEAR_MS,
    MONTH_MS,
    DAY_MS,
    WEEK_MS,
    pad,
    setToEndOfDay,
    toLocaleISOString
  } from "$lib/time";
  import chroma from "chroma-js";
  import { appStore } from "$lib/store";
  import DateRange from "$lib/Components/DateRange.svelte";
  import debounce from "debounce";

  export let chartType: "bar" | "line" | "scatter" = "bar";
  export let divContainerClass = "mb-16";
  export let height = "140pt";
  export let stepsInMinutes = 30;
  export let showLegend = false;
  export let showRangeSelection = false;
  export let initialFrom: Date = new Date(Date.now() - DAY_MS * 2);
  export let updateIntervalInMinutes: number | null = null;
  export let title = `Imports / ${stepsInMinutes} min`;
  export let axes: Axis[] = [{ label: "Docs", types: ["imports"] }];
  export let isStacked = false;
  export let showModeToggle = false;
  export let colors: string[] | undefined = undefined;
  export let source: Source | null = null;

  type Axis = {
    label: string;
    types: StatisticType[];
  };
  type StatisticsMode = "diagram" | "table";
  type Source = {
    id: number;
    isFeed: boolean;
  };

  let from: Date = initialFrom;
  let to: Date = new Date();
  let error: ErrorDetails | null = null;
  let chartComponentRef: any;
  let chart: any;
  let isLoading = false;
  let stats: StatisticGroup = {};
  let intervalID: ReturnType<typeof setInterval> | null;
  let stepsInMilliseconds = 1000 * 60 * stepsInMinutes;
  let mode: StatisticsMode = "diagram";
  let abortController: AbortController | undefined = undefined;
  const basicButtonClass = "py-1 px-3";
  const buttonClass = `${basicButtonClass} bg-white hover:bg-gray-100`;
  const pressedButtonClass = `${basicButtonClass} bg-gray-200 hover:!bg-gray-100 dark:bg-gray-500 dark:hover:!bg-gray-600 dark:text-white text-black`;
  const updateInterval = 1000 * 60 * (updateIntervalInMinutes ?? 0);
  const categoryColors = [
    "#AA0000",
    "#E69F00",
    "#56B4E9",
    "#009E73",
    "#F0E442",
    "#0072B2",
    "#D55E00",
    "#CC79A7"
  ];
  const rangeColors = ["#ddd", "#FFEFB0", "#E6A776", "#CD5D3A", "#B41500"];

  let darkMode = $appStore.app.isDarkMode;

  $: {
    if ($appStore.app.isDarkMode !== darkMode) {
      darkMode = $appStore.app.isDarkMode;
      updateChartColors();
    }
  }

  $: types = axes.map((axis) => axis.types).flat();
  $: datasets = Object.keys(stats).map((key: string, index: number) => {
    let label = getLabelForKey(key);
    const yAxisID = axes.findIndex((axis) => axis.types.includes(key as StatisticType));
    const color = getColor(index);
    return {
      label: label,
      data: stats[key]?.map((s) => {
        return { x: s[0], y: s[1] };
      }),
      borderWidth: chartType === "line" ? 2 : 0,
      backgroundColor: chartType === "line" ? chroma(color).brighten(1.4).hex() : color,
      borderColor: color,
      fill: true,
      pointBackgroundColor: color,
      yAxisID: `y${yAxisID > 0 ? yAxisID : ""}`
    };
  });

  const getColor = (index: number) => {
    let color;
    if (colors) {
      color = colors[index];
    } else if (
      types.length === 1 &&
      !types.includes("importFailures") &&
      !types.includes("totals")
    ) {
      if (types.includes("critical")) {
        color = rangeColors[index];
      } else {
        color = "#3D6090";
      }
    } else {
      color = categoryColors[index];
    }
    return color;
  };

  const isToday = (date: Date) => {
    const today = new Date();
    return (
      date.getDate() === today.getDate() &&
      date.getMonth() === today.getMonth() &&
      date.getFullYear() === today.getFullYear()
    );
  };

  const loadStats = async () => {
    isLoading = true;
    error = null;
    let response: any;
    const toParameter = isToday(to) ? new Date(Date.now() + HOUR_MS) : setToEndOfDay(to);
    const newStats: StatisticGroup = {};
    if (types.includes("imports")) {
      response = await fetchBasicStatistic(
        from,
        toParameter,
        stepsInMilliseconds,
        "imports",
        source?.id,
        source?.isFeed,
        abortController
      );
      if (response.ok) {
        Object.assign(newStats, response.value);
      } else {
        error = response.error;
      }
    }
    if (types.includes("importFailures") || types.includes("importFailuresCombined")) {
      response = await fetchImportFailuresStatistic(
        from,
        toParameter,
        stepsInMilliseconds,
        source?.id,
        source?.isFeed,
        abortController
      );
      if (response.ok) {
        if (types.includes("importFailuresCombined")) {
          Object.assign(newStats, mergeImportFailureStatistics(response.value));
        } else {
          Object.assign(newStats, response.value);
        }
      } else {
        error = response.error;
      }
    }
    if (types.includes("cve")) {
      response = await fetchBasicStatistic(
        from,
        toParameter,
        stepsInMilliseconds,
        "cve",
        source?.id,
        source?.isFeed,
        abortController
      );
      if (response.ok) {
        Object.assign(newStats, response.value);
      } else {
        error = response.error;
      }
    }
    if (types.includes("critical")) {
      const critStats = await getCriticalStatistic(toParameter);
      if (!critStats?.message) {
        Object.assign(newStats, critStats);
      }
    }
    if (types.includes("totals")) {
      response = await fetchTotals(from, toParameter, stepsInMilliseconds, false, abortController);
      if (response.ok) {
        Object.assign(newStats, response.value);
      } else {
        error = response.error;
      }
    }
    stats = newStats;
    isLoading = false;
  };

  const getCriticalStatistic = async (to: Date): Promise<ErrorDetails | undefined> => {
    const response = await fetchBasicStatistic(
      from,
      to,
      stepsInMilliseconds,
      "critical",
      source?.id,
      source?.isFeed,
      abortController
    );
    if (response.ok) {
      const crit: any = response.value.critical;
      if (crit) {
        const critStats: any = {
          cvss_null: [],
          cvss_None: [],
          cvss_Low: [],
          cvss_Medium: [],
          cvss_High: []
        };
        for (let i = 0; i < crit.length; i++) {
          const date = crit[i][0];
          const entries = crit[i][1];
          const counts: any = {
            cvss_null: 0,
            cvss_None: 0,
            cvss_Low: 0,
            cvss_Medium: 0,
            cvss_High: 0
          };
          const keys = Object.keys(critStats);
          // Iterate through the values of one point of time
          if (entries) {
            type NumberOfDocs = number;
            type CritCount = [number | null, NumberOfDocs];
            entries.forEach((entry: CritCount) => {
              const numberOfDocs = entry[1];
              const cvss = entry?.[0];
              if (cvss) {
                const cvssTextualRating: CVSSTextualRating = getCVSSTextualRating(cvss);
                counts[`cvss_${cvssTextualRating}`] =
                  counts[`cvss_${cvssTextualRating}`] + numberOfDocs;
              } else {
                counts["cvss_null"] = counts["cvss_null"] + numberOfDocs;
              }
            });
            keys.forEach((key) => {
              critStats[key].push([date, counts[key]]);
            });
          } else {
            keys.forEach((key) => {
              critStats[key].push([date, 0]);
            });
          }
        }
        return critStats;
      }
    } else {
      error = response.error;
    }
  };

  const setMode = (newMode: StatisticsMode) => {
    mode = newMode;
  };

  const updateOptions = () => {
    chart.options.scales.x.min = from;
    let maxTo = to;
    let diff = to.getTime() - from.getTime();
    if (diff >= YEAR_MS) {
      maxTo.setMonth(maxTo.getMonth() + 1);
    } else if (diff >= MONTH_MS) {
      maxTo.setDate(maxTo.getDate() + 2);
    } else if (isToday(maxTo)) {
      maxTo = new Date(Date.now() + HOUR_MS * 0);
    } else {
      maxTo = setToEndOfDay(new Date(to.getTime()));
    }
    chart.options.scales.x.max = maxTo;
  };

  const updateData = async () => {
    await loadStats();
    chart.data.datasets = datasets;
  };

  const updateChart = async () => {
    updateOptions();
    await updateData();
    chart.update();
  };

  // Source: https://stackoverflow.com/questions/6117814/get-week-of-year-in-javascript-like-in-php/6117889#6117889
  function getWeekNumber(d: Date) {
    // Copy date so don't modify original
    d = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()));
    // Set to nearest Thursday: current date + 4 - current day number
    // Make Sunday's day number 7
    d.setUTCDate(d.getUTCDate() + 4 - (d.getUTCDay() || 7));
    // Get first day of year
    var yearStart = new Date(Date.UTC(d.getUTCFullYear(), 0, 1));
    // Calculate full weeks to nearest Thursday
    var weekNo = Math.ceil(((d.getTime() - yearStart.getTime()) / 86400000 + 1) / 7);
    // Return array of year and week number
    return weekNo;
  }

  const createLabelForXAxis = (date: Date): string | undefined => {
    let label = "";
    const paddedMonth = pad(date.getMonth() + 1);
    const paddedDate = pad(date.getDate());
    const paddedHours = pad(date.getHours());
    const paddedMinutes = pad(date.getMinutes());
    let diff = to.getTime() - from.getTime();
    if (diff >= YEAR_MS) {
      label = `${date.getFullYear()}-${paddedMonth}`;
    } else if (diff > MONTH_MS + 3 * DAY_MS) {
      label = `${date.getFullYear()}-${paddedMonth}-${paddedDate}`;
    } else if (diff >= MONTH_MS) {
      label = `${date.getFullYear()}-W${getWeekNumber(date)}`;
    } else if (diff == WEEK_MS) {
      label = `${date.getFullYear()}-${paddedMonth}-${paddedDate}`;
    } else {
      label = `${paddedHours}:${paddedMinutes}`;
    }
    return label;
  };

  const getCurrentChartLabelColors = () => {
    return darkMode
      ? { lineColor: "#414955", scaleLabelColor: "#bcbfc3", legendLabelColor: "white" }
      : { lineColor: "#e5e5e5", scaleLabelColor: "#666666", legendLabelColor: "black" };
  };

  const updateChartColors = () => {
    const styleColors = getCurrentChartLabelColors();
    for (let key of Object.keys(chart.options.scales)) {
      chart.options.scales[key].border.color = styleColors.lineColor;
      chart.options.scales[key].grid.color = styleColors.lineColor;
      chart.options.scales[key].ticks.color = styleColors.scaleLabelColor;
      chart.options.scales[key].title.color = styleColors.scaleLabelColor;
    }
    chart.options.plugins.legend.labels.generateLabels(chart);
    chart.update();
  };

  const initChart = () => {
    const { lineColor, scaleLabelColor, ..._ } = getCurrentChartLabelColors();
    chart = new Chart(chartComponentRef, {
      type: chartType,
      data: {
        datasets
      },
      options: {
        maintainAspectRatio: false,
        aspectRatio: 1,
        elements: {
          point: {
            radius: 4
          }
        },
        plugins: {
          legend: {
            display: showLegend
          },
          tooltip: {
            callbacks: {
              label: function (context: any) {
                const label = context.dataset.label;
                const addition = ["None", "Low", "Medium", "High"].includes(label)
                  ? ` (${getCVSSTextualRatingDescription(label)})`
                  : "";
                if (context.formattedValue && context.dataset.label) {
                  return `${context.dataset.label}${addition}: ${context.formattedValue}`;
                }
                return "";
              },
              title: (tooltipItems: any[]) => {
                const start: any = tooltipItems[0].dataset.data[tooltipItems[0].dataIndex];
                const end: any = tooltipItems[0].dataset.data[tooltipItems[0].dataIndex + 1];
                if (chartType === "bar") {
                  return `${toLocaleISOString(start.x)}${end ? " - " : ""}${end ? toLocaleISOString(end.x) : ""}`;
                } else {
                  return `${toLocaleISOString(start.x)}`;
                }
              }
            }
          }
        },
        scales: {
          x: {
            border: { color: lineColor },
            type: "time",
            grid: {
              display: true,
              drawOnChartArea: false,
              drawTicks: true,
              tickLength: 6,
              tickWidth: 2,
              color: lineColor
            },
            stacked: isStacked,
            ticks: {
              callback: (tickValue: string | number, _index: number, _ticks: any[]): string => {
                return createLabelForXAxis(new Date(tickValue)) ?? "";
              },
              color: scaleLabelColor
            },
            time: {
              // Overwrite to keep exact time.
              parser: (v: unknown): number => {
                if (v instanceof Date) return v.getTime();
                return 0;
              }
            },
            afterBuildTicks: (axis: any) => {
              const labelColor = getCurrentChartLabelColors().scaleLabelColor;
              const newTicks: any[] = [];
              const firstProperty = Object.keys(stats)[0];
              stats?.[firstProperty]?.forEach((stat, index) => {
                if (stepsInMilliseconds >= HOUR_MS || index % 8 === 0) {
                  newTicks.push({
                    value: stat[0].getTime(),
                    major: false,
                    label: toLocaleISOString(stat[0]),
                    color: labelColor
                  });
                }
              });
              axis.ticks = newTicks;
            }
          },
          y: {
            border: { color: lineColor },
            beginAtZero: true,
            stacked: isStacked,
            title: {
              display: axes[0].label.length > 0,
              text: axes[0].label,
              color: scaleLabelColor
            },
            ticks: { color: scaleLabelColor },
            grid: { color: lineColor }
          }
        }
      }
    });
    if (axes[1]) {
      const showLabel = axes[1].label.length > 0;
      chart.options.scales.y1 = {
        border: { color: lineColor },
        beginAtZero: true,
        grid: {
          drawOnChartArea: false, // only want the grid lines for one axis to show up
          color: lineColor
        },
        ticks: { color: scaleLabelColor },
        title: { display: showLabel, text: axes[1].label, color: scaleLabelColor },
        position: "right"
      };
    }
    // Remove "Crit" from legend labels because otherwise it would appear in front of every crit label
    // which would be too much "noise".
    chart.options.plugins.legend.labels.generateLabels = (chart: any) => {
      const labelColor = getCurrentChartLabelColors().legendLabelColor;
      const items: any[] = [];
      chart.legend.legendItems.forEach((item: any, index: number) => {
        const datasetMeta = chart.getDatasetMeta(item.datasetIndex);
        if (datasetMeta.label) {
          const label = datasetMeta.label.replace("cvss_", "");
          items.push({
            text: label,
            datasetIndex: index,
            fillStyle: getColor(index),
            hidden: datasetMeta.hidden,
            fontColor: labelColor
          });
        }
      });
      return items;
    };
  };

  onMount(async () => {
    from = initialFrom;
    to = new Date();
    await loadStats();
    if (!chartComponentRef) {
      return;
    }
    initChart();
    if (updateIntervalInMinutes) {
      intervalID = setInterval(async () => {
        if (!isLoading) {
          updateChart();
        }
      }, updateInterval);
    }
    updateOptions();
    chart.update();
  });

  onDestroy(() => {
    if (intervalID) {
      clearInterval(intervalID);
    }
    if (chart) {
      chart.destroy();
    }
  });

  // Fit steps to selected time range so the bars don't become to thin.
  const updateSteps = () => {
    let diff = to.getTime() - from.getTime();
    if (diff >= YEAR_MS) {
      stepsInMilliseconds = MONTH_MS;
    } else if (diff >= MONTH_MS) {
      stepsInMilliseconds = WEEK_MS;
    } else if (diff >= WEEK_MS) {
      stepsInMilliseconds = DAY_MS;
    } else {
      stepsInMilliseconds = HOUR_MS;
    }
    updateChart();
  };

  // In case of month or year we need some padding so the last month/year is not cut-off.
  const selectPredefinedRange = (range: string) => {
    const newFrom = new Date();
    const newTo = new Date();
    let diff = 1;
    stepsInMilliseconds = HOUR_MS;
    if (range === "month") {
      diff = 30;
    }
    if (range === "year") {
      diff = 365;
    }
    newFrom.setDate(newFrom.getDate() - diff);
    from = newFrom;
    to = newTo;
    updateSteps();
    updateChart();
  };

  const abortRequests = () => {
    if (abortController) {
      abortController.abort();
    }
  };

  const delayedUpdate = debounce(() => {
    abortController = new AbortController();
    updateSteps();
  }, 600);

  const onSelectedDate = () => {
    abortRequests();
    delayedUpdate();
  };
</script>

<div class={divContainerClass}>
  <div class="flex flex-col gap-4">
    <div class="flex gap-6">
      <h3>{title}</h3>
      {#if showModeToggle}
        <ButtonGroup>
          <Button
            class={mode === "diagram" ? pressedButtonClass : buttonClass}
            on:click={() => setMode("diagram")}><i class="bx bx-bar-chart"></i></Button
          >
          <Button
            class={mode === "table" ? pressedButtonClass : buttonClass}
            on:click={() => setMode("table")}><i class="bx bx-table"></i></Button
          >
        </ButtonGroup>
      {/if}
      {#if isLoading}
        <div class:invisible={!isLoading} class={isLoading ? "loadingFadeIn" : ""}>
          Loading ...
          <Spinner color="gray" size="4"></Spinner>
        </div>
      {/if}
    </div>
    <ErrorMessage {error}></ErrorMessage>
    <div hidden={mode === "table"} class="border px-2">
      <div style:height>
        <canvas bind:this={chartComponentRef}></canvas>
      </div>
    </div>
    {#if mode === "table"}
      <StatsTable {stats}></StatsTable>
    {/if}
    {#if showRangeSelection}
      <div class="my-2 flex flex-wrap items-end justify-start gap-4 md:justify-center">
        <DateRange on:change={onSelectedDate} bind:from bind:to></DateRange>
        <ButtonGroup class="h-fit">
          <Button
            on:click={() => {
              selectPredefinedRange("day");
            }}>Day</Button
          >
          <Button
            on:click={() => {
              selectPredefinedRange("month");
            }}>Month</Button
          >
          <Button
            on:click={() => {
              selectPredefinedRange("year");
            }}>Year</Button
          >
        </ButtonGroup>
      </div>
    {/if}
  </div>
</div>
