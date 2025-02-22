package main

import (
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/business/routing"
	events2 "github.com/antinvestor/service-files/service/events"
	"github.com/antinvestor/service-files/service/queue"
	"github.com/antinvestor/service-files/service/storage/datastore"
	"github.com/antinvestor/service-files/service/storage/models"
	"github.com/antinvestor/service-files/service/storage/provider"

	"github.com/gorilla/handlers"
	"github.com/pitabwire/frame"
	"github.com/sirupsen/logrus"
)

func main() {

	serviceName := "service_files"

	cfg, err := frame.ConfigFromEnv[config.FilesConfig]()
	if err != nil {
		logrus.WithError(err).Fatal("could not process configs")
		return
	}
	ctx, sysService := frame.NewService(serviceName, frame.Config(&cfg))
	defer sysService.Stop(ctx)

	log := sysService.L(ctx)

	serviceOptions := []frame.Option{frame.Datastore(ctx)}

	if cfg.DoDatabaseMigrate() {

		sysService.Init(serviceOptions...)

		err = sysService.DB(ctx, false).Exec(`
			CREATE EXTENSION IF NOT EXISTS pg_search;
			CREATE EXTENSION IF NOT EXISTS pg_analytics;
			CREATE EXTENSION IF NOT EXISTS pg_ivm;
			CREATE EXTENSION IF NOT EXISTS vector;
			CREATE EXTENSION IF NOT EXISTS postgis;
			CREATE EXTENSION IF NOT EXISTS postgis_topology;
			CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;
			CREATE EXTENSION IF NOT EXISTS postgis_tiger_geocoder;
		`).Error
		if err != nil {
			log.Fatalf("main -- Failed to create extensions: %v", err)
		}

		err := sysService.MigrateDatastore(ctx, cfg.GetDatabaseMigrationPath(),
			&models.MediaMetadata{}, &models.MediaAudit{})
		if err != nil {
			log.Fatalf("main -- Could not migrate successfully because : %v", err)
		}

		return
	}

	storageProvider, err := provider.GetStorageProvider(ctx, &cfg)
	if err != nil {
		log.Fatalf("main -- Could not setup or access storage because : %v", err)
	}

	jwtAudience := cfg.Oauth2JwtVerifyAudience
	if jwtAudience == "" {
		jwtAudience = serviceName
	}

	metadataStore, err := datastore.NewMediaDatabase(sysService)
	if err != nil {
		log.Fatalf("main -- failed to setup storage because : %v", err)
	}

	router := routing.SetupMatrixRoutes(sysService, metadataStore, storageProvider)

	authServiceHandlers := handlers.RecoveryHandler(
		handlers.PrintRecoveryStack(true))(
		sysService.AuthenticationMiddleware(router, jwtAudience, cfg.Oauth2JwtVerifyIssuer))

	defaultServer := frame.HttpHandler(authServiceHandlers)
	serviceOptions = append(serviceOptions, defaultServer)

	events := frame.RegisterEvents(
		events2.NewAuditSaveHandler(sysService),
		events2.NewMetadataSaveHandler(sysService),
	)

	serviceOptions = append(serviceOptions, events)

	thumbnailQueueHandler := queue.NewThumbnailQueueHandler(sysService, metadataStore, storageProvider)
	thumbnailGenerateQueue := frame.RegisterSubscriber(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL, 2, &thumbnailQueueHandler)
	thumbnailGeneratePublish := frame.RegisterPublisher(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL)
	serviceOptions = append(serviceOptions, thumbnailGenerateQueue, thumbnailGeneratePublish)

	sysService.Init(serviceOptions...)

	log.WithField("server http port", cfg.HttpServerPort).
		WithField("server grpc port", cfg.GrpcServerPort).
		Info(" Initiating server operations")

	err = sysService.Run(ctx, "")
	if err != nil {
		log.Fatalf("main -- Could not run Server : %v", err)
	}

}
