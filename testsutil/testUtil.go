package testsutil

import (
	"context"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/events"
	"github.com/antinvestor/service-files/service/queue"
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

	err := frame.ConfigFillFromEnv(cfg)
	if err != nil {
		return nil, nil, nil, err
	}

	cfg.LogLevel = "debug"
	cfg.RunServiceSecurely = false
	cfg.ServerPort = ""

	ctx, srv := frame.NewServiceWithContext(ctx, name,
		frame.Config(cfg),
		frame.Datastore(ctx),
		frame.NoopDriver())

	thumbnailQueueHandler := queue.NewThumbnailQueueHandler(srv)

	thumbnailQueue := frame.RegisterSubscriber(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL, 2, &thumbnailQueueHandler)
	thumbnailQueuePublisher := frame.RegisterPublisher(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL)

	serviceOptions := []frame.Option{
		thumbnailQueue, thumbnailQueuePublisher,
		frame.RegisterEvents(events.NewAuditSaveHandler(srv), events.NewMetadataSaveHandler(srv)),
	}

	srv.Init(serviceOptions...)

	err = srv.Run(ctx, "")
	if err != nil {
		return nil, nil, nil, err
	}

	return ctx, srv, func() {

		srv.Stop(ctx)
	}, nil
}
