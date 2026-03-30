package main

import (
	"context"
	"fmt"
	"net/http"

	_ "embed"

	"buf.build/gen/go/antinvestor/files/connectrpc/go/files/v1/filesv1connect"
	filespb "buf.build/gen/go/antinvestor/files/protocolbuffers/go/files/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/common"
	"github.com/antinvestor/common/permissions"
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
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/security/authorizer"
	connectInterceptors "github.com/pitabwire/frame/security/interceptors/connect"
	framehttp "github.com/pitabwire/frame/security/interceptors/httptor"
	"github.com/pitabwire/util"
)

//go:embed files.openapi.yaml
var apiSpecFile []byte

func main() {

	ctx := context.Background()

	cfg, err := config.LoadWithOIDC[aconfig.FilesConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServerName = "service_file"
	}

	if err = cfg.Normalise(); err != nil {
		util.Log(ctx).WithError(err).Fatal("invalid configuration")
	}

	if err = validateEncryptionConfig(&cfg); err != nil {
		util.Log(ctx).WithError(err).Fatal("invalid encryption configuration")
	}

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithConfig(&cfg), frame.WithDatastore())

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
		repository.NewMultipartUploadRepository(ctx, dbPool, workManager),
		repository.NewMultipartUploadPartRepository(ctx, dbPool, workManager),
		repository.NewFileVersionRepository(ctx, dbPool, workManager),
		repository.NewRetentionPolicyRepository(ctx, dbPool, workManager),
		repository.NewFileRetentionRepository(ctx, dbPool, workManager),
		repository.NewStorageStatsRepository(ctx, dbPool, workManager),
	)
	if err != nil {
		log.WithError(err).Fatal("main -- failed to setup storage")
	}

	mediaService := business.NewMediaService(metadataStore, storageProvider)

	sm := svc.SecurityManager()
	authzMiddleware := authz.NewMiddleware(sm.GetAuthorizer(ctx), metadataStore)

	fileServer := handler.NewFileServer(svc, mediaService, authzMiddleware, metadataStore, storageProvider)

	auth := sm.GetAuthorizer(ctx)

	// Layer 1: TenancyAccessChecker verifies caller can access the partition.
	tenancyAccessChecker := authorizer.NewTenancyAccessChecker(auth, authz.NamespaceTenancyAccess)
	tenancyAccessInterceptor := connectInterceptors.NewTenancyAccessInterceptor(tenancyAccessChecker)

	// Layer 2: FunctionAccessInterceptor enforces per-RPC permissions from proto annotations.
	sd := filespb.File_files_v1_files_proto.Services().ByName("FilesService")
	procMap := permissions.BuildProcedureMap(sd)
	functionChecker := authorizer.NewFunctionChecker(auth, permissions.ForService(sd).Namespace)
	functionAccessInterceptor := connectInterceptors.NewFunctionAccessInterceptor(functionChecker, procMap)

	defaultInterceptorList, err := connectInterceptors.DefaultList(ctx, sm.GetAuthenticator(ctx), tenancyAccessInterceptor, functionAccessInterceptor)
	if err != nil {
		log.WithError(err).Fatal("main -- could not create default interceptors")
	}

	connectPath, connectHandler := filesv1connect.NewFilesServiceHandler(
		fileServer, connect.WithInterceptors(defaultInterceptorList...))

	mediaRouter := routing.SetupMediaRoutes(svc, metadataStore, storageProvider, mediaService, authzMiddleware)

	mux := http.NewServeMux()
	mux.Handle(connectPath, connectHandler)
	mux.Handle("/openapi.yaml", common.NewOpenAPIHandler(apiSpecFile, nil))
	mux.Handle("/v1/media/", framehttp.AuthenticationMiddleware(
		framehttp.TenancyAccessMiddleware(mediaRouter, tenancyAccessChecker),
		sm.GetAuthenticator(ctx)))

	defaultServer := frame.WithHTTPHandler(mux)
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
