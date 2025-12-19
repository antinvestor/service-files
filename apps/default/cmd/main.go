package main

import (
	"context"
	"net/http"

	"buf.build/gen/go/antinvestor/files/connectrpc/go/files/v1/filesv1connect"
	aconfig "github.com/antinvestor/service-files/apps/default/config"
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

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithConfig(&cfg), frame.WithRegisterServerOauth2Client(), frame.WithDatastore())

	log := svc.Log(ctx)

	// Create repositories for the database
	dbManager := svc.DatastoreManager()
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	workManager := svc.WorkManager()

	// Handle database migration if requested
	if handleDatabaseMigration(ctx, dbManager, cfg) {
		return
	}

	storageProvider, err := provider.GetStorageProvider(ctx, &cfg)
	if err != nil {
		log.WithError(err).Fatal("main -- Could not setup or access storage")
	}

	mediaRepo := repository.NewMediaRepository(ctx, dbPool, workManager)
	auditRepo := repository.NewMediaAuditRepository(ctx, dbPool, workManager)

	metadataStore, err := connection.NewMediaDatabase(workManager, mediaRepo)
	if err != nil {
		log.WithError(err).Fatal("main -- failed to setup storage")
	}

	// Create business service
	mediaService := business.NewMediaService(metadataStore, storageProvider)

	// Create Connect RPC handler
	fileServer := handler.NewFileServer(svc, mediaService, metadataStore, storageProvider)

	publicRouter := routing.SetupApiSpecRoute(svc)

	router := routing.SetupMatrixRoutes(svc, metadataStore, storageProvider, mediaService)

	// Setup Connect RPC routes
	_, connectHandler := filesv1connect.NewFilesServiceHandler(fileServer)

	// Add Connect router to the public router
	publicRouter.Handle("/rpc/", http.StripPrefix("/rpc", connectHandler))

	sm := svc.SecurityManager()

	authServiceHandlers := handlers.RecoveryHandler(
		handlers.PrintRecoveryStack(true))(
		framehttp.AuthenticationMiddleware(router, sm.GetAuthenticator(ctx)))

	publicRouter.Handle("/", authServiceHandlers)

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

	// Startup service
	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("main -- Could not run Server : %v", err)
	}

}

// handleDatabaseMigration performs database migration if configured to do so.
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
