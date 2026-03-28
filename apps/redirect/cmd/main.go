package main

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/redirect/config"
	"github.com/antinvestor/service-files/apps/redirect/gen/redirect/v1/redirectv1connect"
	"github.com/antinvestor/service-files/apps/redirect/service/business"
	"github.com/antinvestor/service-files/apps/redirect/service/handler"
	"github.com/antinvestor/service-files/apps/redirect/service/models"
	"github.com/pitabwire/frame"
	fconfig "github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	connectinterceptors "github.com/pitabwire/frame/security/interceptors/connect"
	"github.com/pitabwire/util"
)

func main() {
	tmpCtx := context.Background()

	cfg, err := fconfig.LoadWithOIDC[config.RedirectConfig](tmpCtx)
	if err != nil {
		util.Log(tmpCtx).WithError(err).Fatal("could not load config")
		return
	}

	ctx, svc := frame.NewServiceWithContext(tmpCtx,
		frame.WithConfig(&cfg),
		frame.WithDatastore(),
		frame.WithInMemoryCache(handler.CacheName),
	)

	log := util.Log(ctx)

	dbManager := svc.DatastoreManager()

	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	if dbPool == nil {
		log.Fatal("database pool is nil")
		return
	}

	if cfg.DoDatabaseMigrate() {
		migrationPool := dbManager.GetPool(ctx, datastore.DefaultMigrationPoolName)
		if migrationPool == nil {
			migrationPool = dbPool
		}
		err = dbManager.Migrate(ctx, migrationPool, cfg.GetDatabaseMigrationPath(),
			models.Link{}, models.Click{})
		if err != nil {
			log.WithError(err).Fatal("could not migrate")
			return
		}
		return
	}

	rawCache, ok := svc.GetRawCache(handler.CacheName)
	if !ok {
		log.Fatal("redirect cache not available")
		return
	}

	linkBiz, err := business.NewLinkBusiness(dbPool)
	if err != nil {
		log.WithError(err).Fatal("could not create link business")
		return
	}

	// Fast redirect handler with batched async click recording via Frame cache.
	// This is the PUBLIC path — no authentication required.
	redirectHandler := handler.NewRedirectHandler(linkBiz, rawCache, dbPool)
	redirectHandler.Start(ctx)

	// Connect RPC handler for link management — AUTHENTICATED via OIDC interceptors.
	sm := svc.SecurityManager()
	defaultInterceptorList, err := connectinterceptors.DefaultList(ctx, sm.GetAuthenticator(ctx))
	if err != nil {
		log.WithError(err).Fatal("could not create default interceptors")
		return
	}

	implementation := &handler.RedirectServer{
		DBPool:     dbPool,
		RedirectHd: redirectHandler,
	}

	_, connectHandler := redirectv1connect.NewRedirectServiceHandler(
		implementation,
		connect.WithInterceptors(defaultInterceptorList...),
	)

	// Build the HTTP mux.
	mux := http.NewServeMux()

	// Redirect route is public — seamless, open, no auth.
	mux.Handle("/r/", redirectHandler)

	// Connect RPC management API — protected by OIDC auth interceptors.
	mux.Handle("/redirect.v1.RedirectService/", connectHandler)

	svc.Init(ctx, frame.WithHTTPHandler(mux))

	log.Info("redirect service starting")

	err = svc.Run(ctx, "")

	// Graceful shutdown: drain click workers after the HTTP server stops.
	redirectHandler.Stop()

	if err != nil {
		log.WithError(err).Fatal("could not run server")
	}
}
