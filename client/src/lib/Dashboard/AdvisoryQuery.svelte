<!--
 This file is Free Software under the Apache-2.0 License
 without warranty, see README.md and LICENSES/Apache-2.0.txt for details.

 SPDX-License-Identifier: Apache-2.0

 SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
 Software-Engineering: 2024 Intevation GmbH <https://intevation.de>
-->

<script lang="ts">
  import { push } from "svelte-spa-router";
  import { appStore } from "$lib/store";
  import SectionHeader from "$lib/SectionHeader.svelte";
  import { request } from "$lib/request";
  import { getErrorDetails, type ErrorDetails } from "$lib/Errors/error";
  import { onMount } from "svelte";
  import ErrorMessage from "$lib/Errors/ErrorMessage.svelte";
  import Activity from "./Activity.svelte";
  import { getPublisher } from "$lib/publisher";
  import { Spinner } from "flowbite-svelte";
  import { getRelativeTime } from "$lib/time";
  import SsvcBadge from "$lib/Advisories/SSVC/SSVCBadge.svelte";
  import ShowMoreButton from "./ShowMoreButton.svelte";

  export let storedQuery: any;
  let documents: any[] | null = null;
  let newDocumentsError: ErrorDetails | null;
  let isLoading = false;
  const ignoredColumns = [
    "id",
    "title",
    "comments",
    "critical",
    "cvss_v3_score",
    "cvss_v2_score",
    "publisher",
    "tracking_id",
    "state",
    "versions",
    "ssvc",
    "recent"
  ];

  const compareCrit = (a: any, b: any) => {
    if (!b.critical || a.critical > b.critical) {
      return -1;
    } else if (!a.critical || a.critical < b.critical) {
      return 1;
    }
    return 0;
  };

  const loadDocuments = async () => {
    const orders = storedQuery.orders?.join(" ") ?? "";
    let fetchColumns = [...(storedQuery.columns ?? [])];
    let requiredColumns = ["id", "tracking_id", "publisher"];
    for (let c of requiredColumns) {
      if (!fetchColumns.includes(c)) {
        fetchColumns.push(c);
      }
    }
    const response = await request(
      `/api/documents?columns=${fetchColumns.join(" ")}&advisories=true&query=${storedQuery.query}&limit=6&orders=${orders}`,
      "GET"
    );
    if (response.ok) {
      documents = (await response.content.documents)?.sort(compareCrit) ?? [];
    } else if (response.error) {
      newDocumentsError = getErrorDetails(`Could not load new documents.`, response);
    }
  };

  onMount(async () => {
    isLoading = true;
    await loadDocuments();
    isLoading = false;
  });

  const openDocument = (doc: any) => {
    push(`/advisories/${doc.publisher}/${doc.tracking_id}/documents/${doc.id}`);
  };
</script>

{#if $appStore.app.isUserLoggedIn}
  <div class="flex flex-col gap-4 md:w-[46%] md:max-w-[46%]">
    <SectionHeader title={storedQuery.description}></SectionHeader>
    <div class="grid grid-cols-[repeat(auto-fit,_minmax(200pt,_1fr))] gap-6">
      {#if isLoading}
        <div class:invisible={!isLoading} class={isLoading ? "loadingFadeIn" : ""}>
          Loading ...
          <Spinner color="gray" size="4"></Spinner>
        </div>
      {/if}
      {#if documents}
        {#if documents.length > 0}
          {#each documents as doc}
            <Activity on:click={() => openDocument(doc)}>
              <div slot="top-left">
                {#if doc.critical}
                  <span class:text-red-500={Number(doc.critical) > 5.0}>
                    {doc.critical}
                  </span>
                {/if}
              </div>
              <span slot="top-right" class="ml-auto">{getPublisher(doc.publisher)}</span>
              <div class="text-black dark:text-white">{doc.title ?? "Title: undefined"}</div>
              <div class="text-sm text-gray-700 dark:text-gray-400">{doc.tracking_id}</div>
              <div slot="bottom-left" class="flex items-center gap-4 text-slate-400">
                {#if doc.comments !== undefined}
                  <div class="flex items-center gap-1">
                    <i class="bx bx-comment"></i>
                    <span>{doc.comments}</span>
                  </div>
                {/if}
                {#if doc.versions !== undefined}
                  <div class="flex items-center gap-1">
                    <i class="bx bx-collection"></i>
                    <span>{doc.versions}</span>
                  </div>
                {/if}
                {#if doc.ssvc}
                  <SsvcBadge vector={doc.ssvc}></SsvcBadge>
                {/if}
              </div>
              <div slot="bottom-right" class="text-slate-400">
                {#if doc.recent !== undefined}
                  <span>{getRelativeTime(new Date(doc.recent))}</span>
                {/if}
              </div>
              <div slot="bottom-bottom">
                {#if Object.keys(doc).filter((k) => !ignoredColumns.includes(k)).length > 0}
                  <div class="my-2 rounded-sm border p-2 text-xs text-gray-800">
                    {#each Object.keys(doc).sort() as key}
                      {#if !ignoredColumns.includes(key) && doc[key] !== undefined && doc[key] !== null}
                        <div>{key}: {doc[key]}</div>
                      {/if}
                    {/each}
                  </div>
                {/if}
              </div>
            </Activity>
          {/each}
        {:else}
          <div class="text-gray-600 dark:text-gray-400">No matching advisories found.</div>
        {/if}
      {/if}
    </div>
    <ShowMoreButton id={storedQuery.id}></ShowMoreButton>
    <ErrorMessage error={newDocumentsError}></ErrorMessage>
  </div>
{/if}
