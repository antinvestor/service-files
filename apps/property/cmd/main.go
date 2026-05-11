package main

import (
	"context"
	"log/slog"

	propertyv1connect "buf.build/gen/go/antinvestor/property/connectrpc/go/v1/propertyv1connect"
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
		slog.Error("could not load config", "error", err)
		return
	}

	ctx, svc := frame.NewServiceWithContext(tmpCtx,
		frame.WithConfig(&cfg),
		frame.WithDatastore(),
	)

	dbManager := svc.DatastoreManager()

	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	if dbPool == nil {
		slog.Error("database pool is nil")
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
			slog.Error("could not migrate database", "error", err)
			return
		}
		return
	}

	sm := svc.SecurityManager()

	implementation := &handlers.PropertyServer{
		DBPool: dbPool,
	}

	// TenancyTxInterceptor opens a request-scoped transaction after auth
	// has populated the claims, publishes app.tenant_id + app.partition_id
	// from the claims via set_config, and binds the transaction to the
	// request context. Repository code then calls pool.DB(ctx, _) and gets
	// the bound tx transparently; tenancy is enforced by Row-Level Security
	// at the database layer.
	tenancyTxInterceptor := connectinterceptors.NewTenancyTxInterceptor(dbPool)

	defaultInterceptorList, err := connectinterceptors.DefaultList(ctx, sm.GetAuthenticator(ctx), tenancyTxInterceptor)
	if err != nil {
		slog.Error("could not create default interceptors", "error", err)
		return
	}

	_, serverHandler := propertyv1connect.NewPropertyServiceHandler(
		implementation, connect.WithInterceptors(defaultInterceptorList...))

	svc.Init(ctx, frame.WithHTTPHandler(serverHandler))

	err = svc.Run(ctx, "")
	if err != nil {
		slog.Error("could not run server", "error", err)
	}
}
