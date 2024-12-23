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
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ISDuBA/ISDuBA/pkg/database/query"
	"github.com/ISDuBA/ISDuBA/pkg/models"
)

func (c *Controller) isCommentingAllowed(ctx *gin.Context, state models.Workflow) bool {
	// Check if we are in a state in which commenting is allowed.
	switch state {
	case models.ReadWorkflow, models.AssessingWorkflow:
		return true
	case models.ReviewWorkflow:
		return c.hasAnyRole(ctx, models.Reviewer, models.Editor, models.Admin)
	case models.ArchivedWorkflow:
		return c.hasAnyRole(ctx, models.Editor, models.Admin)
	case models.DeleteWorkflow:
		return c.hasAnyRole(ctx, models.Admin)
	default:
		return false
	}
}

func (c *Controller) createComment(ctx *gin.Context) {
	docID, ok := parse(ctx, toInt64, ctx.Param("document"))
	if !ok {
		return
	}

	expr := c.andTLPExpr(ctx, query.FieldEqInt("id", docID))
	builder := query.SQLBuilder{}
	builder.CreateWhere(expr)

	var (
		exists            bool
		commentingAllowed bool
		forbidden         bool
		commentator       = c.currentUser(ctx)
		message, _        = ctx.GetPostForm("message")
		now               = time.Now().UTC()
		commentID         *int64
	)

	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			tx, err := conn.BeginTx(rctx, pgx.TxOptions{})
			if err != nil {
				return err
			}
			defer tx.Rollback(rctx)

			stateSQL := `SELECT state, docs.tracking_id, docs.publisher ` +
				`FROM documents docs JOIN advisories ads ` +
				`ON (docs.tracking_id, docs.publisher) = (ads.tracking_id, ads.publisher) ` +
				` WHERE ` + builder.WhereClause

			var (
				stateS     string
				trackingID string
				publisher  string
			)
			if err := tx.QueryRow(rctx, stateSQL, builder.Replacements...).Scan(
				&stateS, &trackingID, &publisher,
			); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
				return err
			}
			exists = true

			state := models.Workflow(stateS)
			commentingAllowed = c.isCommentingAllowed(ctx, state)
			if !commentingAllowed {
				return nil
			}

			logEvent := func(event models.Event, state models.Workflow) error {
				const eventSQL = `INSERT INTO events_log ` +
					`(event, state, time, actor, documents_id, comments_id) ` +
					`VALUES($1::events, $2::workflow, $3, $4, $5, $6)`
				_, err := tx.Exec(
					rctx, eventSQL, string(event), string(state), now, commentator, docID, commentID)
				return err
			}

			// Switch to assessing state if we are not in.
			if state == models.ReadWorkflow {
				// Check if the transition is allowed to user.
				roles := models.ReadWorkflow.TransitionsRoles(models.AssessingWorkflow)
				if !c.hasAnyRole(ctx, roles...) {
					forbidden = true
					return nil
				}

				// Switch to assessing state.
				const assessingStateSQL = `UPDATE advisories SET state = 'assessing' ` +
					`WHERE (tracking_id, publisher) = ($1, $2)`
				if _, err := tx.Exec(rctx, assessingStateSQL, trackingID, publisher); err != nil {
					return err
				}

				// Log that we switched state.
				if err := logEvent(models.StateChangeEvent, models.AssessingWorkflow); err != nil {
					return err
				}
			}

			// Now insert the comment itself
			const insertSQL = `INSERT INTO comments ` +
				`(documents_id, time, commentator, message) ` +
				`VALUES ($1, $2, $3, $4) ` +
				`RETURNING id`

			if err := tx.QueryRow(
				rctx, insertSQL,
				docID, now, commentator, message,
			).Scan(&commentID); err != nil {
				return err
			}

			// Log that we created a comment
			if err := logEvent(models.AddCommentEvent, models.AssessingWorkflow); err != nil {
				return err
			}

			return tx.Commit(rctx)
		}, 0,
	); err != nil {
		slog.Error("database error", "err", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	switch {
	case !exists:
		ctx.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
	case !commentingAllowed:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid state to comment"})
	case forbidden:
		ctx.JSON(http.StatusForbidden, gin.H{"error": "user not allowed to change state"})
	default:
		ctx.JSON(http.StatusCreated, gin.H{
			"id":          commentID,
			"time":        now,
			"commentator": commentator,
		})
	}
}

func (c *Controller) updateComment(ctx *gin.Context) {
	commentID, ok := parse(ctx, toInt64, ctx.Param("id"))
	if !ok {
		return
	}

	expr := c.andTLPExpr(ctx, query.FieldEqInt("com.id", commentID))
	builder := query.SQLBuilder{}
	builder.CreateWhere(expr)

	var (
		exists            bool
		now               = time.Now().UTC()
		commentator       = ctx.GetString("uid")
		commentingAllowed bool
		message, _        = ctx.GetPostForm("message")
	)
	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			tx, err := conn.BeginTx(rctx, pgx.TxOptions{})
			if err != nil {
				return err
			}
			defer tx.Rollback(rctx)
			stateSQL := `SELECT state ` +
				`FROM advisories ads JOIN documents docs ` +
				`ON (docs.tracking_id, docs.publisher) = (ads.tracking_id, ads.publisher) ` +
				`JOIN comments com ` +
				`ON com.documents_id = docs.id` +
				` WHERE ` + builder.WhereClause

			var stateS string
			if err := tx.QueryRow(rctx, stateSQL, builder.Replacements...).Scan(
				&stateS); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
				return err
			}
			exists = true

			state := models.Workflow(stateS)
			commentingAllowed = c.isCommentingAllowed(ctx, state)
			if !commentingAllowed {
				return nil
			}

			const updateSQL = `UPDATE comments ` +
				`SET message = $1 ` +
				`WHERE id = $2 AND commentator = $3 ` +
				`RETURNING documents_id`

			var docID int64
			switch err := tx.QueryRow(
				rctx, updateSQL, message, commentID, commentator,
			).Scan(&docID); {
			case errors.Is(err, pgx.ErrNoRows):
				exists = false
				return nil
			case err != nil:
				return err
			}
			exists = true

			const eventSQL = `INSERT INTO events_log ` +
				`(event, state, time, actor, documents_id, comments_id) ` +
				`VALUES('change_comment', ` +
				`(SELECT state FROM advisories ads JOIN documents docs ` +
				`ON (ads.tracking_id, ads.publisher) = (docs.tracking_id, docs.publisher) ` +
				`WHERE docs.id = $3), ` +
				`$1, $2, $3, $4)`

			actor := c.currentUser(ctx)
			if _, err := tx.Exec(rctx, eventSQL, now, actor, docID, commentID); err != nil {
				return err
			}

			return tx.Commit(rctx)
		}, 0,
	); err != nil {
		slog.Error("database error", "err", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
		return
	}
	if !commentingAllowed {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid state to comment"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{})
}

type comment struct {
	DocumentID  int64     `json:"document_id"`
	ID          int64     `json:"id"`
	Time        time.Time `json:"time"`
	Commentator string    `json:"commentator"`
	Message     string    `json:"message"`
}

func (c *Controller) viewComment(ctx *gin.Context) {
	id, ok := parse(ctx, toInt64, ctx.Param("id"))
	if !ok {
		return
	}

	expr := c.andTLPExpr(ctx, query.FieldEqInt("comments.id", id))

	builder := query.SQLBuilder{}

	fetchSQL := `SELECT documents_id, time, commentator, message ` +
		`FROM comments JOIN documents ON comments.documents_id = documents.id ` +
		`WHERE ` + builder.CreateWhere(expr)

	post := comment{ID: id}
	switch err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			return conn.QueryRow(rctx, fetchSQL, builder.Replacements...).Scan(
				&post.DocumentID,
				&post.Time,
				&post.Commentator,
				&post.Message)
		}, 0); {
	case errors.Is(err, pgx.ErrNoRows):
		ctx.JSON(http.StatusNotFound, gin.H{"error": "comment post not found"})
	case err != nil:
		slog.Error("database error while fetching comment post", "err", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	default:
		ctx.JSON(http.StatusOK, &post)
	}
}

func (c *Controller) viewComments(ctx *gin.Context) {
	var key models.AdvisoryKey
	if err := ctx.ShouldBindUri(&key); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if key.Publisher == "" || key.TrackingID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing publisher or tracking_id"})
		return
	}

	expr := c.andTLPExpr(ctx,
		query.FieldEqString("tracking_id", key.TrackingID).And(
			query.FieldEqString("publisher", key.Publisher)))

	builder := query.SQLBuilder{}
	builder.CreateWhere(expr)

	var comments []comment
	var exists bool

	if err := c.db.Run(
		ctx.Request.Context(),
		func(rctx context.Context, conn *pgxpool.Conn) error {
			existsSQL := `SELECT exists(SELECT FROM documents WHERE ` +
				builder.WhereClause + `)`
			if err := conn.QueryRow(
				rctx, existsSQL, builder.Replacements...).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return nil
			}
			fetchSQL := `SELECT id, documents_id, time, commentator, message FROM comments ` +
				`WHERE documents_id in (SELECT id FROM documents WHERE ` +
				builder.WhereClause +
				` ) ORDER BY time DESC`
			rows, _ := conn.Query(rctx, fetchSQL, builder.Replacements...)
			var err error
			comments, err = pgx.CollectRows(
				rows,
				func(row pgx.CollectableRow) (comment, error) {
					var com comment
					err := row.Scan(&com.ID, &com.DocumentID, &com.Time, &com.Commentator, &com.Message)
					com.Time = com.Time.UTC()
					return com, err
				})
			return err
		}, 0,
	); err != nil {
		slog.Error("database error", "err", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	ctx.JSON(http.StatusOK, comments)
}
