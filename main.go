package main

import (
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/openapi"
	"github.com/antinvestor/service-files/service/business/storage"
	"github.com/antinvestor/service-files/service/models"
	"github.com/antinvestor/service-files/service/queue"
	"github.com/gorilla/handlers"
	"github.com/pitabwire/frame"
	"github.com/sirupsen/logrus"
)

func main() {

	serviceName := "service_files"

	var cfg config.FilesConfig
	err := frame.ConfigProcess("", &cfg)
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
		err := sysService.MigrateDatastore(ctx, cfg.GetDatabaseMigrationPath(),
			&models.File{}, &models.FileAudit{})
		if err != nil {
			log.Fatalf("main -- Could not migrate successfully because : %v", err)
		}

		return
	}

	storageProvider, err := storage.GetStorageProvider(ctx, &cfg)
	if err != nil {
		log.Fatalf("main -- Could not setup or access storage because : %v", err)
	}

	jwtAudience := cfg.Oauth2JwtVerifyAudience
	if jwtAudience == "" {
		jwtAudience = serviceName
	}

	apiService := openapi.NewApiV1Service(sysService, storageProvider)
	apiController := openapi.NewDefaultApiController(apiService)
	router := openapi.NewRouter(apiController)

	authServiceHandlers := handlers.RecoveryHandler(
		handlers.PrintRecoveryStack(true))(
		sysService.AuthenticationMiddleware(router, jwtAudience, cfg.Oauth2JwtVerifyIssuer))

	defaultServer := frame.HttpHandler(authServiceHandlers)
	serviceOptions = append(serviceOptions, defaultServer)

	fileSyncQueueHandler := queue.NewFileQueueHandler(sysService)
	fileSyncQueue := frame.RegisterSubscriber(cfg.QueueFileSyncName, cfg.QueueFileSyncURL, 2, &fileSyncQueueHandler)
	fileSyncQueueP := frame.RegisterPublisher(cfg.QueueFileSyncName, cfg.QueueFileSyncURL)
	serviceOptions = append(serviceOptions, fileSyncQueue, fileSyncQueueP)

	fileAuditSyncQueueHandler := queue.NewFileAuditQueueHandler(sysService)
	fileAuditSyncQueue := frame.RegisterSubscriber(cfg.QueueFileAuditSyncName, cfg.QueueFileAuditSyncURL, 2, &fileAuditSyncQueueHandler)
	fileAuditSyncQueueP := frame.RegisterPublisher(cfg.QueueFileAuditSyncName, cfg.QueueFileAuditSyncURL)
	serviceOptions = append(serviceOptions, fileAuditSyncQueue, fileAuditSyncQueueP)

	sysService.Init(serviceOptions...)

	log.WithField("server http port", cfg.HttpServerPort).
		WithField("server grpc port", cfg.GrpcServerPort).
		Info(" Initiating server operations")

	err = sysService.Run(ctx, "")
	if err != nil {
		log.Fatalf("main -- Could not run Server : %v", err)
	}

}
