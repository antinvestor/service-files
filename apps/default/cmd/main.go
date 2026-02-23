package main

import (
	"context"
	"fmt"
	"net/http"

	"buf.build/gen/go/antinvestor/files/connectrpc/go/files/v1/filesv1connect"
	aconfig "github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/authz"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/events"
	"github.com/antinvestor/service-files/apps/default/service/handler"
	"github.com/antinvestor/service-files/apps/default/service/handler/routing"
	"github.com/antinvestor/service-files/apps/default/service/queue"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/gorilla/handlers"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	framehttp "github.com/pitabwire/frame/security/interceptors/httptor"
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

	if err = cfg.Normalise(); err != nil {
		util.Log(ctx).WithError(err).Fatal("invalid configuration")
	}

	if err = validateEncryptionConfig(&cfg); err != nil {
		util.Log(ctx).WithError(err).Fatal("invalid encryption configuration")
	}

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithConfig(&cfg), frame.WithRegisterServerOauth2Client(), frame.WithDatastore())

	log := svc.Log(ctx)

	dbManager := svc.DatastoreManager()
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	workManager := svc.WorkManager()

	if handleDatabaseMigration(ctx, dbManager, cfg) {
		return
	}

	storageProvider, err := provider.GetStorageProvider(ctx, &cfg)
	if err != nil {
		log.WithError(err).Fatal("main -- Could not setup or access storage")
	}

	mediaRepo := repository.NewMediaRepository(ctx, dbPool, workManager)
	auditRepo := repository.NewMediaAuditRepository(ctx, dbPool, workManager)

	metadataStore, err := connection.NewMediaDatabase(
		workManager,
		mediaRepo,
		repository.NewMultipartUploadRepository(ctx, dbPool),
		repository.NewMultipartUploadPartRepository(ctx, dbPool),
		repository.NewFileVersionRepository(ctx, dbPool),
		repository.NewRetentionPolicyRepository(ctx, dbPool),
		repository.NewFileRetentionRepository(ctx, dbPool),
		repository.NewStorageStatsRepository(ctx, dbPool),
	)
	if err != nil {
		log.WithError(err).Fatal("main -- failed to setup storage")
	}

	mediaService := business.NewMediaService(metadataStore, storageProvider)

	sm := svc.SecurityManager()
	authorizer := sm.GetAuthorizer(ctx)
	authzMiddleware := authz.NewMiddleware(authorizer, metadataStore)

	fileServer := handler.NewFileServer(svc, mediaService, authzMiddleware, metadataStore, storageProvider)

	publicRouter := routing.SetupApiSpecRoute(svc)

	router := routing.SetupMatrixRoutes(svc, metadataStore, storageProvider, mediaService, authzMiddleware)

	_, connectHandler := filesv1connect.NewFilesServiceHandler(fileServer)

	connectAuthHandler := handlers.RecoveryHandler(
		handlers.PrintRecoveryStack(true))(
		framehttp.AuthenticationMiddleware(connectHandler, sm.GetAuthenticator(ctx)))

	connectProcedures := []string{
		filesv1connect.FilesServiceUploadContentProcedure,
		filesv1connect.FilesServiceCreateContentProcedure,
		filesv1connect.FilesServiceGetContentProcedure,
		filesv1connect.FilesServiceGetContentOverrideNameProcedure,
		filesv1connect.FilesServiceGetContentThumbnailProcedure,
		filesv1connect.FilesServiceGetUrlPreviewProcedure,
		filesv1connect.FilesServiceGetConfigProcedure,
		filesv1connect.FilesServiceSearchMediaProcedure,
	}
	for _, procedure := range connectProcedures {
		publicRouter.Handle(procedure, connectAuthHandler).Methods(http.MethodPost, http.MethodGet, http.MethodOptions)
	}

	authServiceHandlers := handlers.RecoveryHandler(
		handlers.PrintRecoveryStack(true))(
		framehttp.AuthenticationMiddleware(router, sm.GetAuthenticator(ctx)))

	publicRouter.Handle("/*", authServiceHandlers)

	defaultServer := frame.WithHTTPHandler(publicRouter)
	serviceOptions := []frame.Option{defaultServer, frame.WithRegisterEvents(
		events.NewAuditSaveHandler(auditRepo),
		events.NewMetadataSaveHandler(mediaRepo),
	)}

	thumbnailQueueHandler := queue.NewThumbnailQueueHandler(svc, metadataStore, storageProvider)
	thumbnailGenerateQueue := frame.WithRegisterSubscriber(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL, &thumbnailQueueHandler)
	thumbnailGeneratePublish := frame.WithRegisterPublisher(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL)
	serviceOptions = append(serviceOptions, thumbnailGenerateQueue, thumbnailGeneratePublish)

	svc.Init(ctx, serviceOptions...)

	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("main -- Could not run Server : %v", err)
	}

}

func handleDatabaseMigration(
	ctx context.Context,
	dbManager datastore.Manager,
	cfg aconfig.FilesConfig,
) bool {

	if cfg.DoDatabaseMigrate() {

		err := repository.Migrate(ctx, dbManager, cfg.GetDatabaseMigrationPath())
		if err != nil {
			util.Log(ctx).WithError(err).Fatal("main -- Could not migrate successfully")
		}
		return true
	}
	return false
}

func validateEncryptionConfig(cfg *aconfig.FilesConfig) error {
	if cfg.EnvStorageEncryptionPhrase == "" {
		return fmt.Errorf("ENCRYPTION_PHRASE must be set for private file encryption")
	}
	if len(cfg.EnvStorageEncryptionPhrase) != 32 {
		return fmt.Errorf("ENCRYPTION_PHRASE must be 32 bytes for AES-256-GCM")
	}
	return nil
}
