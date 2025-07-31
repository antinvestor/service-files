package testsutil

import (
	"context"

	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/events"
	"github.com/antinvestor/service-files/service/queue"
	"github.com/antinvestor/service-files/service/storage/datastore"
	"github.com/antinvestor/service-files/service/storage/provider"
	"github.com/pitabwire/frame"
)

const PostgresqlDbImage = "paradedb/paradedb:latest"

func GetTestService(ctx context.Context, name string) (context.Context, *frame.Service, func(), error) {

	cfg, err := frame.ConfigFromEnv[config.FilesConfig]()
	if err != nil {
		return nil, nil, nil, err
	}
	return GetTestServiceWithConfig(ctx, name, &cfg)
}
func GetTestServiceWithConfig(ctx context.Context, name string, cfg *config.FilesConfig) (context.Context, *frame.Service, func(), error) {

	cfg.LogLevel = "debug"
	cfg.RunServiceSecurely = false
	cfg.ServerPort = ""

	ctx, srv := frame.NewServiceWithContext(ctx, name,
		frame.WithConfig(cfg),
		frame.WithDatastore(),
		frame.WithNoopDriver())

	storageProvider, err := provider.GetStorageProvider(ctx, cfg)
	if err != nil {
		return nil, nil, nil, err
	}

	metadataStore, err := datastore.NewMediaDatabase(srv)
	if err != nil {
		return nil, nil, nil, err
	}

	thumbnailQueueHandler := queue.NewThumbnailQueueHandler(srv, metadataStore, storageProvider)

	thumbnailQueue := frame.WithRegisterSubscriber(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL, &thumbnailQueueHandler)
	thumbnailQueuePublisher := frame.WithRegisterPublisher(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL)

	serviceOptions := []frame.Option{
		thumbnailQueue, thumbnailQueuePublisher,
		frame.WithRegisterEvents(events.NewAuditSaveHandler(srv), events.NewMetadataSaveHandler(srv)),
	}

	srv.Init(ctx, serviceOptions...)

	err = srv.Run(ctx, "")
	if err != nil {
		return nil, nil, nil, err
	}

	return ctx, srv, func() {

		srv.Stop(ctx)
	}, nil
}
