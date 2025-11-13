package main

import (
	"context"

	aconfig "github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business/routing"
	events3 "github.com/antinvestor/service-files/apps/default/service/events"
	"github.com/antinvestor/service-files/apps/default/service/queue"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/gorilla/handlers"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/security/interceptors/http"
	"github.com/pitabwire/util"
)

func main() {

	ctx := context.Background()

	cfg, err := config.LoadWithOIDC[aconfig.FilesConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServerName = "service_files"
	}

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithConfig(&cfg), frame.WithRegisterServerOauth2Client())

	log := svc.Log(ctx)

	serviceOptions := []frame.Option{frame.WithDatastore()}

	// Handle database migration if requested
	if handleDatabaseMigration(ctx, svc, cfg, log) {
		return
	}

	storageProvider, err := provider.GetStorageProvider(ctx, &cfg)
	if err != nil {
		log.WithError(err).Fatal("main -- Could not setup or access storage")
	}

	metadataStore, err := connection.NewMediaDatabase(svc)
	if err != nil {
		log.WithError(err).Fatal("main -- failed to setup storage")
	}

	publicRouter := routing.SetupApiSpecRoute(svc)

	router := routing.SetupMatrixRoutes(svc, metadataStore, storageProvider)

	sm := svc.SecurityManager()

	authServiceHandlers := handlers.RecoveryHandler(
		handlers.PrintRecoveryStack(true))(
		http.AuthenticationMiddleware(router, sm.GetAuthenticator(ctx)))

	publicRouter.Handle("/", authServiceHandlers)

	defaultServer := frame.WithHTTPHandler(publicRouter)
	serviceOptions = append(serviceOptions, defaultServer)

	events := frame.WithRegisterEvents(
		events3.NewAuditSaveHandler(svc),
		events3.NewMetadataSaveHandler(svc),
	)

	serviceOptions = append(serviceOptions, events)

	thumbnailQueueHandler := queue.NewThumbnailQueueHandler(svc, metadataStore, storageProvider)
	thumbnailGenerateQueue := frame.WithRegisterSubscriber(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL, &thumbnailQueueHandler)
	thumbnailGeneratePublish := frame.WithRegisterPublisher(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL)
	serviceOptions = append(serviceOptions, thumbnailGenerateQueue, thumbnailGeneratePublish)

	svc.Init(ctx, serviceOptions...)

	log.WithField("server http port", cfg.HTTPPort()).
		WithField("server grpc port", cfg.HTTPPort()).
		Info(" Initiating server operations")

	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("main -- Could not run Server : %v", err)
	}

}

// handleDatabaseMigration performs database migration if configured to do so.
func handleDatabaseMigration(
	ctx context.Context,
	svc *frame.Service,
	cfg aconfig.FilesConfig,
	log *util.LogEntry,
) bool {
	serviceOptions := []frame.Option{frame.WithDatastore()}

	if cfg.DoDatabaseMigrate() {
		svc.Init(ctx, serviceOptions...)

		err := repository.Migrate(ctx, svc.DatastoreManager(), cfg.GetDatabaseMigrationPath())
		if err != nil {
			log.WithError(err).Fatal("main -- Could not migrate successfully")
		}
		return true
	}
	return false
}
