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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ISDuBA/ISDuBA/pkg/models"
	"github.com/ISDuBA/ISDuBA/pkg/sources"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type custom struct {
	ID            int64                         `json:"id,omitempty"`
	Name          string                        `json:"name,omitempty"`
	Attention     *bool                         `json:"attention,omitempty"`
	Subscriptions []sources.SourceSubscriptions `json:"subscriptions,omitempty"`
}

type argumentedAggregator struct {
	Aggregator json.RawMessage `json:"aggregator"`
	Custom     custom          `json:"custom"`
}

// aggregatorProxy is an endpoint the aggregator metadata for a URL.
//
//	@Summary		Returns the aggregator metadata.
//	@Description	Fetches and returns the aggregator metadata for the specified URL.
//	@Param			url	query	string	true	"Aggregator URL"
//	@Produce		json
//	@Success		200	{object}	argumentedAggregator
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/aggregator [get]
func (c *Controller) aggregatorProxy(ctx *gin.Context) {
	url := ctx.Query("url")
	ca, err := c.am.Cache.GetAggregator(url, c.cfg)
	if err != nil {
		models.SendError(ctx, http.StatusBadRequest, err)
		return
	}
	// search in database
	const sql = `SELECT ` +
		`id, name, (checksum_ack < checksum_updated) AS attention ` +
		`FROM aggregators WHERE url = $1`
	var (
		id        int64
		name      string
		attention bool
	)
	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			return conn.QueryRow(rctx, sql, url).Scan(&id, &name, &attention)
		}, 0,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("fetching aggregator failed", "err", err)
		models.SendError(ctx, http.StatusInternalServerError, err)
		return
	}
	custom := custom{
		Subscriptions: c.sm.Subscriptions(ca.SourceURLs()),
	}
	if name != "" {
		custom.ID = id
		custom.Name = name
		custom.Attention = &attention
	}
	aAgg := argumentedAggregator{
		Aggregator: ca.Raw,
		Custom:     custom,
	}
	ctx.JSON(http.StatusOK, &aAgg)
}

// viewAggregators is an endpoint that returns all configured aggregators.
//
//	@Summary		Returns all aggregators.
//	@Description	Returns all aggregators that are configured.
//	@Produce		json
//	@Success		200	{array}	web.viewAggregators.aggregator
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/aggregators [get]
func (c *Controller) viewAggregators(ctx *gin.Context) {
	type aggregator struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		URL       string `json:"url"`
		Active    bool   `json:"active"`
		Attention bool   `json:"attention"`
	}
	var list []aggregator
	const sql = `SELECT ` +
		`id, name, url, active, (checksum_ack < checksum_updated) AS attention ` +
		`FROM aggregators ORDER by name`
	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			rows, _ := conn.Query(rctx, sql)
			var err error
			list, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (aggregator, error) {
				var a aggregator
				err := row.Scan(&a.ID, &a.Name, &a.URL, &a.Active, &a.Attention)
				return a, err
			})
			return err
		}, 0,
	); err != nil {
		slog.Error("fetching aggregators failed", "error", err)
		models.SendError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, list)
}

// viewAggregator is an endpoint that returns the specified.
//
//	@Summary		Returns the aggregator.
//	@Description	Returns metadata and configuration of the specified aggregator.
//	@Param			id	path	int	true	"Aggregator ID"
//	@Produce		json
//	@Success		200	{object}	argumentedAggregator
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		404	{object}	models.Error
//	@Failure		404	{object}	models.Error	"not found"
//	@Failure		500	{object}	models.Error
//	@Router			/aggregators/{id} [get]
func (c *Controller) viewAggregator(ctx *gin.Context) {
	id, ok := parse(ctx, toInt64, ctx.Param("id"))
	if !ok {
		return
	}
	var (
		name      string
		url       string
		active    bool
		attention bool
	)
	const sql = `SELECT ` +
		`name, url, active, (checksum_ack < checksum_updated) AS attention ` +
		`FROM aggregators WHERE id = $1`
	switch err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			return conn.QueryRow(rctx, sql, id).Scan(&name, &url, &active, &attention)
		}, 0,
	); {
	case errors.Is(err, pgx.ErrNoRows):
		models.SendErrorMessage(ctx, http.StatusNotFound, "not found")
		return
	case err != nil:
		slog.Error("fetching aggregator failed", "err", err)
		models.SendError(ctx, http.StatusInternalServerError, err)
		return
	}
	ca, err := c.am.Cache.GetAggregator(url, c.cfg)
	if err != nil {
		models.SendError(ctx, http.StatusBadRequest, err)
		return
	}
	aAgg := argumentedAggregator{
		Aggregator: ca.Raw,
		Custom: custom{
			ID:            id,
			Name:          name,
			Attention:     &attention,
			Subscriptions: c.sm.Subscriptions(ca.SourceURLs()),
		},
	}
	ctx.JSON(http.StatusOK, &aAgg)
}

// createAggregator is an endpoint that creates an aggregator with the specified configuration.
//
//	@Summary		Creates an aggregator.
//	@Description	Creates an aggregator with specified configuration.
//	@Param			name	formData	string	true	"Aggregator name"
//	@Param			url		formData	string	true	"Aggregator URL"
//	@Accept			multipart/form-data
//	@Produce		json
//	@Success		201	{object}	models.ID
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		404	{object}	models.Error
//	@Failure		404	{object}	models.Error	"not found"
//	@Failure		500	{object}	models.Error
//	@Router			/aggregators [post]
func (c *Controller) createAggregator(ctx *gin.Context) {
	var (
		ok     bool
		name   string
		url    string
		active bool
		id     int64
	)
	if name, ok = parse(ctx, notEmpty, ctx.PostForm("name")); !ok {
		return
	}
	if url, ok = parse(ctx, endsWith("/aggregator.json"), ctx.PostForm("url")); !ok {
		return
	}
	activeParam, ok := ctx.GetPostForm("active")
	if ok {
		act, ok := parse(ctx, strconv.ParseBool, activeParam)
		active = act
		if !ok {
			return
		}
	}

	const sql = `INSERT INTO aggregators (name, url, active) VALUES ($1, $2, $3) RETURNING id`
	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			return conn.QueryRow(rctx, sql, name, url, active).Scan(&id)
		}, 0,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			models.SendErrorMessage(ctx, http.StatusBadRequest,
				fmt.Sprintf("not a unique value: %v", err.Error()))
		} else {
			slog.Error("inserting aggregator failed", "error", err)
			models.SendError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	ctx.JSON(http.StatusCreated, models.ID{ID: id})
}

// deleteAggregator is an endpoint that deletes the aggregator with specified ID.
//
//	@Summary		Deletes an aggregator.
//	@Description	Deletes the aggregator configuration with the specified ID.
//	@Param			id	path	int	true	"Aggregator ID"
//	@Produce		json
//	@Success		200	{object}	models.Success	"deleted"
//	@Failure		400	{object}	models.Error	"could not parse id"
//	@Failure		401
//	@Failure		404	{object}	models.Error	"not found"
//	@Failure		500	{object}	models.Error
//	@Router			/aggregators/{id} [delete]
func (c *Controller) deleteAggregator(ctx *gin.Context) {
	id, ok := parse(ctx, toInt64, ctx.Param("id"))
	if !ok {
		return
	}
	const sql = `DELETE FROM aggregators WHERE id = $1`
	var deleted bool
	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			tag, err := conn.Exec(rctx, sql, id)
			deleted = tag.RowsAffected() > 0
			return err
		}, 0,
	); err != nil {
		slog.Error("delete aggregator failed", "error", err)
		models.SendError(ctx, http.StatusInternalServerError, err)
		return
	}
	if deleted {
		models.SendSuccess(ctx, http.StatusOK, "deleted")
	} else {
		models.SendErrorMessage(ctx, http.StatusNotFound, "not found")
	}
}

func (c *Controller) attentionAggregators(ctx *gin.Context) {
	const sql = `SELECT id, name FROM aggregators ` +
		`WHERE checksum_ack < checksum_updated ` +
		`ORDER BY name`
	type attention struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	var list []attention
	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			rows, _ := conn.Query(rctx, sql)
			var err error
			list, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (attention, error) {
				var att attention
				err := row.Scan(&att.ID, &att.Name)
				return att, err
			})
			return err
		}, 0,
	); err != nil {
		slog.Error("fetching aggregator failed", "error", err)
		models.SendError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, list)
}

// updateAggregator is an endpoint that updates the aggregator configuration.
//
//	@Summary		Updates aggregator configuration.
//	@Description	Updates the aggregator configuration.
//	@Param			id			path		int		true	"Aggregator ID"
//	@Param			name		formData	string	false	"Aggregator name"
//	@Param			url			formData	string	false	"Aggregator URL"
//	@Param			active		formData	bool	false	"Aggregator active flag"
//	@Param			attention	formData	bool	false	"Aggregator attention flag"
//	@Accept			multipart/form-data
//	@Produce		json
//	@Success		200	{object}	models.Success
//	@Failure		400	{object}	models.Error
//	@Failure		401
//	@Failure		500	{object}	models.Error
//	@Router			/aggregators/{id} [put]
func (c *Controller) updateAggregator(ctx *gin.Context) {
	const (
		prefix      = `UPDATE aggregators SET `
		suffix      = ` WHERE id = $1`
		sqlAtt      = `checksum_ack = checksum_updated`
		sqlAttTrue  = sqlAtt + ` - interval '1s'`
		sqlAttFalse = sqlAtt
	)
	var (
		values []any
		fields []string
		add    = func(field string, value any) {
			values = append(values, value)
			fields = append(fields, fmt.Sprintf("%s = $%d", field, len(values)))
		}
	)
	id, ok := parse(ctx, toInt64, ctx.Param("id"))
	if !ok {
		return
	}
	values = append(values, id)

	if nameParam, ok := ctx.GetPostForm("name"); ok {
		name, ok := parse(ctx, notEmpty, nameParam)
		if !ok {
			return
		}
		add("name", name)
	}
	if urlParam, ok := ctx.GetPostForm("url"); ok {
		u, ok := parse(ctx, endsWith("/aggregator.json"), urlParam)
		if !ok {
			return
		}
		add("url", u)
	}
	if activeParam, ok := ctx.GetPostForm("active"); ok {
		act, ok := parse(ctx, strconv.ParseBool, activeParam)
		if !ok {
			return
		}
		add("active", act)
	}
	if attentionParam, ok := ctx.GetPostForm("attention"); ok {
		att, ok := parse(ctx, strconv.ParseBool, attentionParam)
		if !ok {
			return
		}
		if att {
			fields = append(fields, sqlAttTrue)
		} else {
			fields = append(fields, sqlAttFalse)
		}
	}

	if len(fields) == 0 {
		models.SendSuccess(ctx, http.StatusOK, "unchanged")
		return
	}

	var changed bool

	updateSQL := prefix + strings.Join(fields, ",") + suffix
	slog.Debug("update aggregators", "sql", updateSQL, "values", values)

	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			tags, err := conn.Exec(rctx, updateSQL, values...)
			if err != nil {
				return err
			}
			changed = tags.RowsAffected() > 0
			return nil
		}, 0,
	); err != nil {
		var pgErr *pgconn.PgError
		// Unique constraint violation
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			models.SendError(ctx, http.StatusBadRequest, err)
			return
		}
		slog.Error("updating aggregator failed", "error", err)
		models.SendError(ctx, http.StatusInternalServerError, err)
		return
	}
	if changed {
		models.SendSuccess(ctx, http.StatusOK, "changed")
	} else {
		models.SendSuccess(ctx, http.StatusOK, "unchanged")
	}
}
