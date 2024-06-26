// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2024 Intevation GmbH <https://intevation.de>

// Package web implements the endpoints of the web server.
package web

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"

	"github.com/ISDuBA/ISDuBA/pkg/config"
	"github.com/ISDuBA/ISDuBA/pkg/database"
	"github.com/ISDuBA/ISDuBA/pkg/ginkeycloak"
	"github.com/ISDuBA/ISDuBA/pkg/models"
)

// Controller binds the endpoints to the internal logic.
type Controller struct {
	cfg *config.Config
	db  *database.DB
}

// NewController returns a new Controller.
func NewController(cfg *config.Config, db *database.DB) *Controller {
	return &Controller{cfg: cfg, db: db}
}

// currentUser returns the current user to be used in database queries.
func (c *Controller) currentUser(ctx *gin.Context) sql.NullString {
	var user sql.NullString
	if !c.cfg.General.AnonymousEventLogging {
		user.String = ctx.GetString("uid")
		user.Valid = true
	}
	return user
}

// Bind return a http handler to be used in a web server.
func (c *Controller) Bind() http.Handler {
	r := gin.New()
	r.Use(sloggin.New(slog.Default()))
	r.Use(gin.Recovery())

	if c.cfg.Web.Static != "" {
		r.Use(static.Serve("/", static.LocalFile(c.cfg.Web.Static, false)))
	}

	kcCfg := c.cfg.Keycloak.Config(extractTLPs)

	authRoles := func(roles ...string) gin.HandlerFunc {
		return ginkeycloak.Auth(ginkeycloak.RoleCheck(roles...), kcCfg)
	}

	var (
		authAdm    = authRoles(models.Admin)
		authIm     = authRoles(models.Importer)
		authEdRe   = authRoles(models.Editor, models.Reviewer)
		authEdReAu = authRoles(models.Editor, models.Reviewer, models.Auditor)
		authEdReAd = authRoles(models.Editor, models.Reviewer, models.Admin)
		authAll    = authRoles(models.Admin, models.Importer, models.Editor,
			models.Reviewer, models.Auditor)
	)

	api := r.Group("/api")

	// Documents
	// Importer can import (POST) documents
	api.POST("/documents", authIm, c.importDocument)
	// Everyone can view (GET) overviewDocuments and viewDocuments?
	api.GET("/documents", authAll /* authEdReAu */, c.overviewDocuments)
	api.GET("/documents/:id", authAll /* authEdReAu */, c.viewDocument)
	// Admin can delete documents
	api.DELETE("/documents/:id", authAdm, c.deleteDocument)

	// Advisories
	api.DELETE("/advisory/:publisher/:trackingid", authAdm, c.deleteAdvisory)

	// Comments
	api.POST("/comments/:document", authEdRe, c.createComment)
	api.PUT("/comments/:id", authEdRe, c.updateComment)
	api.GET("/comments/:document", authEdReAu, c.viewComments)

	// Stored queries
	api.POST("/queries", authAll, c.createStoredQuery)
	api.GET("/queries", authAll, c.listStoredQueries)
	api.GET("/queries/:query", authAll, c.fetchStoredQuery)
	api.PUT("/queries/:query", authAll, c.updateStoredQuery)
	api.DELETE("/queries/:query", authAll, c.deleteStoredQuery)

	// Events
	api.GET("/events/:document", authEdReAu, c.viewEvents)

	// State change
	api.PUT("/status/:publisher/:trackingid/:state", authEdReAd, c.changeStatus)
	api.PUT("/status", authEdReAd, c.changeStatusBulk)

	// SSVC change
	api.PUT("/ssvc/:document", authEdRe, c.changeSSVC)

	// Calculate diff
	api.GET("/diff/:document1/:document2", authEdRe, c.viewDiff)

	// Backend information
	api.GET("/about", authAll, c.about)

	// Visibility information
	api.GET("/view", authAll, c.view)
	return r
}
