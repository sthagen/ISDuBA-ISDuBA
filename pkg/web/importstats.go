// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2024 Intevation GmbH <https://intevation.de>

package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ISDuBA/ISDuBA/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	selectImportStatsSQL = `SELECT ` +
		`date_bin($1, time, $2) AS bucket,` +
		`count(*) AS count ` +
		`FROM downloads ` +
		`%s ` + // placeholder for deeper joins.
		`WHERE time BETWEEN $2 AND $3 ` +
		`%s ` + // placeholder for more filters.
		`GROUP BY bucket ` +
		`ORDER BY bucket`
	selectCVEStatsSQL = `SELECT ` +
		`date_bin($1, time, $2) AS bucket,` +
		`count(distinct cve_id) AS count ` +
		`FROM downloads JOIN documents ON downloads.documents_id = documents.id ` +
		`JOIN documents_cves ON documents.id = documents_cves.documents_id ` +
		`%s ` + // placeholder for deeper joins.
		`WHERE time BETWEEN $2 AND $3 ` +
		`%s ` + // placeholder for more filters.
		`GROUP BY bucket ` +
		`ORDER BY bucket`
	selectCriticalSQL = `SELECT ` +
		`date_bin($1, time, $2) AS bucket,` +
		`critical,` +
		`count(*) AS count ` +
		`FROM downloads JOIN documents ON downloads.documents_id = documents.id ` +
		`%s ` + // placeholder for deeper joins.
		`WHERE time BETWEEN $2 AND $3 ` +
		`%s ` + // placeholder for more filters.
		`GROUP BY bucket, critical ` +
		`ORDER BY bucket, critical`
)

// cveStatsSource is an endpoint that returns statistics for source CVEs.
//
//	@Summary		Returns cve statistics.
//	@Description	Returns cve statistics for the specified source.
//	@Param			id		path	int		true	"Source ID"
//	@Param			from	query	string	false	"Timerange start"
//	@Param			to		query	string	false	"Timerange end"
//	@Param			step	query	string	false	"Time step"
//	@Produce		json
//	@Success		200	{object}	any
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/stats/cve/source/{id} [get]
func (c *Controller) cveStatsSource(ctx *gin.Context) {
	c.importStatsSourceTmpl(ctx, selectCVEStatsSQL, collectBuckets)
}

// cveStatsFeed is an endpoint that returns statistics for feed CVEs.
//
//	@Summary		Returns cve statistics.
//	@Description	Returns cve statistics for the specified feed.
//	@Param			id		path	int		true	"Feed ID"
//	@Param			from	query	string	false	"Timerange start"
//	@Param			to		query	string	false	"Timerange end"
//	@Param			step	query	string	false	"Time step"
//	@Produce		json
//	@Success		200	{object}	any
//	@Failure		401
//	@Failure		400	{object}	models.Error
//	@Failure		500	{object}	models.Error
//	@Router			/stats/cve/feed/{id} [get]
func (c *Controller) cveStatsFeed(ctx *gin.Context) {
	c.importStatsFeedTmpl(ctx, selectCVEStatsSQL, collectBuckets)
}

// cveStatsAllSources is an endpoint that returns statistics from all sources CVEs.
//
//	@Summary		Returns cve statistics.
//	@Description	Returns cve statistics for all sources.
//	@Param			from	query	string	false	"Timerange start"
//	@Param			to		query	string	false	"Timerange end"
//	@Param			step	query	string	false	"Time step"
//	@Produce		json
//	@Success		200	{object}	any
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/stats/cve [get]
func (c *Controller) cveStatsAllSources(ctx *gin.Context) {
	c.importStatsAllSourcesTmpl(ctx, selectCVEStatsSQL, collectBuckets)
}

// importStatsSource is an endpoint that returns import statistics for the source.
//
//	@Summary		Returns import statistics.
//	@Description	Returns import statistics for the specified source.
//	@Param			id		path	int		true	"Source ID"
//	@Param			from	query	string	false	"Timerange start"
//	@Param			to		query	string	false	"Timerange end"
//	@Param			step	query	string	false	"Time step"
//	@Produce		json
//	@Success		200	{object}	any
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/stats/imports/source/{id} [get]
func (c *Controller) importStatsSource(ctx *gin.Context) {
	c.importStatsSourceTmpl(ctx, selectImportStatsSQL, collectBuckets)
}

// importStatsAllSources is an endpoint that returns import statistics for all sources.
//
//	@Summary		Returns import statistics.
//	@Description	Returns import statistics for all sources.
//	@Param			from	query	string	false	"Timerange start"
//	@Param			to		query	string	false	"Timerange end"
//	@Param			step	query	string	false	"Time step"
//	@Produce		json
//	@Success		200	{object}	any
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/stats/imports [get]
func (c *Controller) importStatsAllSources(ctx *gin.Context) {
	c.importStatsAllSourcesTmpl(ctx, selectImportStatsSQL, collectBuckets)
}

// importStatsFeed is an endpoint that returns import statistics for the feed.
//
//	@Summary		Returns import statistics.
//	@Description	Returns import statistics for the specified feed.
//	@Param			id		path	int		true	"Feed ID"
//	@Param			from	query	string	false	"Timerange start"
//	@Param			to		query	string	false	"Timerange end"
//	@Param			step	query	string	false	"Time step"
//	@Produce		json
//	@Success		200	{object}	any
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/stats/imports/feed/{id} [get]
func (c *Controller) importStatsFeed(ctx *gin.Context) {
	c.importStatsFeedTmpl(ctx, selectImportStatsSQL, collectBuckets)
}

// criticalStatsSource is an endpoint that returns critical statistics for the source.
//
//	@Summary		Returns criticality statistics.
//	@Description	Returns criticality statistics for the specified source.
//	@Param			id		path	int		true	"Source ID"
//	@Param			from	query	string	false	"Timerange start"
//	@Param			to		query	string	false	"Timerange end"
//	@Param			step	query	string	false	"Time step"
//	@Produce		json
//	@Success		200	{object}	any
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/stats/critical/source/{id} [get]
func (c *Controller) criticalStatsSource(ctx *gin.Context) {
	c.importStatsSourceTmpl(ctx, selectCriticalSQL, collectCritcalBuckets)
}

// criticalStatsAllSources is an endpoint that returns criticality statistics for all sources.
//
//	@Summary		Returns criticality statistics.
//	@Description	Returns criticality statistics for all sources.
//	@Param			from	query	string	false	"Timerange start"
//	@Param			to		query	string	false	"Timerange end"
//	@Param			step	query	string	false	"Time step"
//	@Produce		json
//	@Success		200	{object}	any
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/stats/critical [get]
func (c *Controller) criticalStatsAllSources(ctx *gin.Context) {
	c.importStatsAllSourcesTmpl(ctx, selectCriticalSQL, collectCritcalBuckets)
}

// criticalStatsFeed is an endpoint that returns crtiticality statistics for the feed.
//
//	@Summary		Returns criticality statistics.
//	@Description	Returns criticality statistics for the specified feed.
//	@Param			id		path	int		true	"Feed ID"
//	@Param			from	query	string	false	"Timerange start"
//	@Param			to		query	string	false	"Timerange end"
//	@Param			step	query	string	false	"Time step"
//	@Produce		json
//	@Success		200	{object}	any
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/stats/critical/feed/{id} [get]
func (c *Controller) criticalStatsFeed(ctx *gin.Context) {
	c.importStatsFeedTmpl(ctx, selectCriticalSQL, collectCritcalBuckets)
}

const importStatsDefaultInterval = 3 * 24 * time.Hour

func (c *Controller) importStatsSourceTmpl(
	ctx *gin.Context,
	sqlTmpl string,
	collectBuckets func(rows pgx.Rows) ([][]any, error),
) {
	sourcesID, ok := parse(ctx, toInt64, ctx.Param("id"))
	if !ok {
		return
	}
	from, to, step, ok := importStatsInterval(ctx, importStatsDefaultInterval)
	if !ok {
		return
	}
	var cond strings.Builder
	cond.WriteString(`AND feeds.sources_id = $4`)
	if !filterImportStats(ctx, &cond) {
		return
	}
	c.serveImportStats(ctx,
		func(rctx context.Context, conn *pgxpool.Conn) (pgx.Rows, error) {
			const joinFeeds = `JOIN feeds ON downloads.feeds_id = feeds.id`
			sql := fmt.Sprintf(sqlTmpl, joinFeeds, cond.String())
			return conn.Query(rctx, sql, step, from, to, sourcesID)
		}, collectBuckets)
}

func (c *Controller) importStatsAllSourcesTmpl(
	ctx *gin.Context,
	sqlTmpl string,
	collectBuckets func(rows pgx.Rows) ([][]any, error),
) {
	from, to, step, ok := importStatsInterval(ctx, importStatsDefaultInterval)
	if !ok {
		return
	}
	var cond strings.Builder
	if !filterImportStats(ctx, &cond) {
		return
	}
	c.serveImportStats(ctx,
		func(rctx context.Context, conn *pgxpool.Conn) (pgx.Rows, error) {
			sql := fmt.Sprintf(sqlTmpl, "", cond.String())
			return conn.Query(rctx, sql, step, from, to)
		}, collectBuckets)
}

func (c *Controller) importStatsFeedTmpl(
	ctx *gin.Context,
	sqlTmpl string,
	collectBuckets func(rows pgx.Rows) ([][]any, error),
) {
	feedID, ok := parse(ctx, toInt64, ctx.Param("id"))
	if !ok {
		return
	}
	from, to, step, ok := importStatsInterval(ctx, importStatsDefaultInterval)
	if !ok {
		return
	}
	var cond strings.Builder
	cond.WriteString(`AND feeds_id = $4`)
	if !filterImportStats(ctx, &cond) {
		return
	}
	c.serveImportStats(ctx,
		func(rctx context.Context, conn *pgxpool.Conn) (pgx.Rows, error) {
			sql := fmt.Sprintf(sqlTmpl, "", cond.String())
			return conn.Query(rctx, sql, step, from, to, feedID)
		}, collectBuckets)
}

func (c *Controller) serveImportStats(
	ctx *gin.Context,
	query func(context.Context, *pgxpool.Conn) (pgx.Rows, error),
	collectBuckets func(rows pgx.Rows) ([][]any, error),
) {
	var list [][]any
	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			rows, _ := query(rctx, conn)
			var err error
			list, err = collectBuckets(rows)
			return err
		}, 0,
	); err != nil {
		slog.Error("Cannot fetch import stats", "error", err)
		models.SendError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, list)
}

func collectBuckets(rows pgx.Rows) ([][]any, error) {
	return pgx.CollectRows(rows,
		func(row pgx.CollectableRow) ([]any, error) {
			var bucket time.Time
			var count int64
			if err := row.Scan(&bucket, &count); err != nil {
				return nil, err
			}
			return []any{bucket.UTC(), count}, nil
		})
}

func collectCritcalBuckets(rows pgx.Rows) ([][]any, error) {
	defer rows.Close()
	var (
		list = [][]any{} // [[bucket, [[critical, count], ...]], ...]
		bins [][]any
		last time.Time
	)
	add := func(bucket time.Time, critical *float64, count int64) {
		if len(bins) == 0 || bucket.Equal(last) {
			bins = append(bins, []any{critical, count})
		} else if len(bins) > 0 {
			list = append(list, []any{last, bins})
			bins = [][]any{{critical, count}}
		}
		last = bucket
	}
	for rows.Next() {
		var (
			bucket   time.Time
			critical *float64
			count    int64
		)
		if err := rows.Scan(&bucket, &critical, &count); err != nil {
			return nil, fmt.Errorf("cannot scan criticals: %w", err)
		}
		add(bucket.UTC(), critical, count)
	}
	if len(bins) > 0 {
		list = append(list, []any{last, bins})
	}
	return list, nil
}

func importStatsInterval(
	ctx *gin.Context,
	diff time.Duration,
) (time.Time, time.Time, time.Duration, bool) {
	var (
		ok       bool
		from, to time.Time
		step     time.Duration
		now      = sync.OnceValue(time.Now)
	)
	if value := ctx.Query("from"); value != "" {
		if from, ok = parse(ctx, parseTime, value); !ok {
			return time.Time{}, time.Time{}, 0, false
		}
	} else {
		from = now().Add(-diff)
	}

	if value := ctx.Query("to"); value != "" {
		if to, ok = parse(ctx, parseTime, value); !ok {
			return time.Time{}, time.Time{}, 0, false
		}
	} else {
		to = now()
	}

	if to.Before(from) {
		to, from = from, to
	}

	if value := ctx.Query("step"); value != "" {
		if step, ok = parse(ctx, time.ParseDuration, value); !ok {
			return time.Time{}, time.Time{}, 0, false
		}
		step = step.Abs()
	} else {
		step = to.Sub(from)
	}
	return from.UTC(), to.UTC(), step, true
}

func filterImportStats(ctx *gin.Context, cond *strings.Builder) bool {
	have := false
	for _, flag := range []string{
		"download_failed",
		"filename_failed",
		"schema_failed",
		"remote_failed",
		"checksum_failed",
		"signature_failed",
		"duplicate_failed",
	} {
		if value := ctx.Query(flag); value != "" {
			v, ok := parse(ctx, strconv.ParseBool, value)
			if !ok {
				return false
			}
			if have {
				cond.WriteString(" OR ")
			} else {
				have = true
				cond.WriteString(" AND (")
			}
			if !v {
				cond.WriteString("NOT ")
			}
			cond.WriteString(flag)
		}
	}
	if have {
		cond.WriteByte(')')
	}
	return true
}
