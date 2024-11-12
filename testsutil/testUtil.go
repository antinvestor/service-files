package testsutil

import (
	"context"
	"github.com/antinvestor/service-files/config"
	"github.com/pitabwire/frame"
)

func GetTestService(name string) (context.Context, *frame.Service, *config.FilesConfig, error) {

	ctx := context.Background()
	dbURL := frame.GetEnv("TEST_DATABASE_URL",
		"postgres://ant:secret@localhost:5425/service_files?sslmode=disable")
	mainDB := frame.DatastoreConnection(ctx, dbURL, false)

	var cfg config.FilesConfig
	err := frame.ConfigProcess("", &cfg)
	if err != nil {
		return nil, nil, nil, err
	}

	fileQueuePublisher := frame.RegisterPublisher(cfg.QueueFileSyncName, cfg.QueueFileSyncURL)

	ctx, service := frame.NewServiceWithContext(ctx, name, frame.Config(&cfg), fileQueuePublisher, mainDB, frame.NoopDriver())
	service.Init()

	return ctx, service, &cfg, nil
}
