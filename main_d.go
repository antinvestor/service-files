package main

import (
	"context"
	"fmt"
	"github.com/antinvestor/files/config"
	"github.com/antinvestor/files/openapi"
	"github.com/antinvestor/files/service"
	storage2 "github.com/antinvestor/files/service/business/storage"
	models2 "github.com/antinvestor/files/service/models"
	"github.com/antinvestor/files/service/queue"
	"github.com/gorilla/handlers"
	"github.com/pitabwire/frame"
	"log"
	"os"
	"strconv"
)

func main() {

	serviceName := "files"

	ctx := context.Background()

	datasource := frame.GetEnv(config.EnvDatabaseUrl, "postgres://ant:@nt@localhost:5423/service_files")
	mainDb := frame.Datastore(ctx, datasource, false)

	readOnlydatasource := frame.GetEnv(config.EnvReplicaDatabaseUrl, datasource)
	readDb := frame.Datastore(ctx, readOnlydatasource, true)

	sysService := frame.NewService(serviceName, mainDb, readDb)

	isMigration, err := strconv.ParseBool(frame.GetEnv(config.EnvMigrate, "false"))
	if err != nil {
		isMigration = false
	}

	stdArgs := os.Args[1:]
	if (len(stdArgs) > 0 && stdArgs[0] == "migrate") || isMigration {

		migrationPath := frame.GetEnv(config.EnvMigrationPath, "./migrations/0001")
		err := sysService.MigrateDatastore(ctx, migrationPath,
			&models2.File{}, &models2.FileAudit{})
		if err != nil {
			log.Fatalf("main -- Could not migrate successfully because : %v", err)
		}

		return
	}

	var serviceOptions []frame.Option

	storageProviderName := frame.GetEnv(config.EnvStorageProvider, "LOCAL")
	storageProvider, err := storage2.GetStorageProvider(ctx, storageProviderName)
	if err != nil {
		log.Fatalf("main -- Could not setup or access storage because : %v", err)
	}

	jwtAudience := frame.GetEnv(config.EnvOauth2JwtVerifyAudience, serviceName)
	jwtIssuer := frame.GetEnv(config.EnvOauth2JwtVerifyIssuer, "")


	apiService := openapi.NewApiV1Service(sysService, storageProvider)

	authServiceHandlers := handlers.RecoveryHandler(
		handlers.PrintRecoveryStack(true))(
		frame.AuthenticationMiddleware(
			openapi.NewDefaultApiController(apiService), jwtAudience, jwtIssuer))

	defaultServer := frame.HttpHandler(authServiceHandlers)
	serviceOptions = append(serviceOptions, defaultServer)


	fileSyncQueueHandler := queue.NewFileQueueHandler(sysService)
	fileSyncQueueURL := frame.GetEnv(config.EnvQueueFileSync, fmt.Sprintf("mem://%s", config.QueueFileSyncName))
	fileSyncQueue := frame.RegisterSubscriber(config.QueueFileSyncName, fileSyncQueueURL, 2, &fileSyncQueueHandler)
	fileSyncQueueP := frame.RegisterPublisher(config.QueueFileSyncName, fileSyncQueueURL)
	serviceOptions = append(serviceOptions, fileSyncQueue, fileSyncQueueP)


	fileAuditSyncQueueHandler := queue.NewFileAuditQueueHandler(sysService)
	fileAuditSyncQueueURL := frame.GetEnv(config.EnvQueueFileAuditSync, fmt.Sprintf("mem://%s", config.QueueFileAuditSyncName))
	fileAuditSyncQueue := frame.RegisterSubscriber(config.QueueFileAuditSyncName, fileAuditSyncQueueURL, 2, &fileAuditSyncQueueHandler)
	fileAuditSyncQueueP := frame.RegisterPublisher(config.QueueFileAuditSyncName, fileAuditSyncQueueURL)
	serviceOptions = append(serviceOptions, fileAuditSyncQueue, fileAuditSyncQueueP)

	sysService.Init(serviceOptions...)

	serverPort := frame.GetEnv(config.EnvServerPort, "7513")

	log.Printf(" main -- Initiating server operations on : %s", serverPort)
	err = sysService.Run(ctx, fmt.Sprintf(":%v", serverPort))
	if err != nil {
		log.Fatalf("main -- Could not run Server : %v", err)
	}

}
