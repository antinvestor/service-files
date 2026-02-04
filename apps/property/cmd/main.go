package main

import (
	"context"
	"log/slog"

	propertyv1connect "buf.build/gen/go/antinvestor/property/connectrpc/go/property/v1/propertyv1connect"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/property/config"
	"github.com/antinvestor/service-files/apps/property/service/handlers"
	"github.com/antinvestor/service-files/apps/property/service/models"
	"github.com/pitabwire/frame"
	fconfig "github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	connectinterceptors "github.com/pitabwire/frame/security/interceptors/connect"
)

func main() {

	tmpCtx := context.Background()

	cfg, err := fconfig.LoadWithOIDC[config.PropertyConfig](tmpCtx)
	if err != nil {
		slog.Error("main -- could not load config", "error", err)
		return
	}

	ctx, svc := frame.NewServiceWithContext(tmpCtx,
		frame.WithConfig(&cfg),
		frame.WithRegisterServerOauth2Client(),
		frame.WithDatastore(),
	)

	dbManager := svc.DatastoreManager()

	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	if dbPool == nil {
		slog.Error("main -- database pool is nil")
		return
	}

	if cfg.DoDatabaseMigrate() {
		migrationPool := dbManager.GetPool(ctx, datastore.DefaultMigrationPoolName)
		if migrationPool == nil {
			migrationPool = dbPool
		}
		err = dbManager.Migrate(ctx, migrationPool, cfg.GetDatabaseMigrationPath(),
			models.PropertyType{}, models.Locality{}, models.PropertyState{},
			models.Property{}, models.Subscription{})
		if err != nil {
			slog.Error("main -- could not migrate", "error", err)
			return
		}
		return
	}

	sm := svc.SecurityManager()

	implementation := &handlers.PropertyServer{
		DBPool: dbPool,
	}

	defaultInterceptorList, err := connectinterceptors.DefaultList(ctx, sm.GetAuthenticator(ctx))
	if err != nil {
		slog.Error("main -- could not create default interceptors", "error", err)
		return
	}

	_, serverHandler := propertyv1connect.NewPropertyServiceHandler(
		implementation, connect.WithInterceptors(defaultInterceptorList...))

	svc.Init(ctx, frame.WithHTTPHandler(serverHandler))

	err = svc.Run(ctx, "")
	if err != nil {
		slog.Error("main -- could not run server", "error", err)
	}
}
