package main

import (
	"context"

	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/business/routing"
	events2 "github.com/antinvestor/service-files/service/events"
	"github.com/antinvestor/service-files/service/queue"
	"github.com/antinvestor/service-files/service/storage/datastore"
	"github.com/antinvestor/service-files/service/storage/provider"
	"github.com/antinvestor/service-files/service/storage/repository"
	"github.com/gorilla/handlers"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/util"
)

func main() {

	serviceName := "service_files"
	ctx := context.Background()

	cfg, err := frame.ConfigFromEnv[config.FilesConfig]()
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	ctx, svc := frame.NewService(serviceName, frame.WithConfig(&cfg))

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

	jwtAudience := cfg.Oauth2JwtVerifyAudience
	if jwtAudience == "" {
		jwtAudience = serviceName
	}

	metadataStore, err := datastore.NewMediaDatabase(svc)
	if err != nil {
		log.WithError(err).Fatal("main -- failed to setup storage")
	}

	router := routing.SetupMatrixRoutes(svc, metadataStore, storageProvider)

	authServiceHandlers := handlers.RecoveryHandler(
		handlers.PrintRecoveryStack(true))(
		svc.AuthenticationMiddleware(router, jwtAudience, cfg.Oauth2JwtVerifyIssuer))

	defaultServer := frame.WithHTTPHandler(authServiceHandlers)
	serviceOptions = append(serviceOptions, defaultServer)

	events := frame.WithRegisterEvents(
		events2.NewAuditSaveHandler(svc),
		events2.NewMetadataSaveHandler(svc),
	)

	serviceOptions = append(serviceOptions, events)

	thumbnailQueueHandler := queue.NewThumbnailQueueHandler(svc, metadataStore, storageProvider)
	thumbnailGenerateQueue := frame.WithRegisterSubscriber(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL, &thumbnailQueueHandler)
	thumbnailGeneratePublish := frame.WithRegisterPublisher(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL)
	serviceOptions = append(serviceOptions, thumbnailGenerateQueue, thumbnailGeneratePublish)

	svc.Init(ctx, serviceOptions...)

	log.WithField("server http port", cfg.HTTPServerPort).
		WithField("server grpc port", cfg.GrpcServerPort).
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
	cfg config.FilesConfig,
	log *util.LogEntry,
) bool {
	serviceOptions := []frame.Option{frame.WithDatastore()}

	if cfg.DoDatabaseMigrate() {
		svc.Init(ctx, serviceOptions...)

		err := repository.Migrate(ctx, svc, cfg.GetDatabaseMigrationPath())
		if err != nil {
			log.WithError(err).Fatal("main -- Could not migrate successfully")
		}
		return true
	}
	return false
}
