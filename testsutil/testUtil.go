package testsutil

import (
	"context"
	"fmt"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/events"
	"github.com/antinvestor/service-files/service/queue"
	"github.com/docker/docker/api/types/container"
	"github.com/pitabwire/frame"
	"github.com/testcontainers/testcontainers-go"
	tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"path/filepath"
	"runtime"
	"time"
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

func setupPostgres(ctx context.Context) (*tcPostgres.PostgresContainer, error) {

	postgresContainer, err := tcPostgres.Run(ctx, PostgresqlDbImage,
		tcPostgres.WithDatabase("service_files"),
		tcPostgres.WithUsername("ant"),
		tcPostgres.WithPassword("s3cr3t"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second)),
	)
	if err != nil {
		return nil, err
	}

	return postgresContainer, nil
}

func setupMigrations(ctx context.Context, networks []string, postgresqlUri string) error {

	_, b, _, _ := runtime.Caller(0)
	basePath := filepath.Join(filepath.Dir(b), "../")

	g := testcontainers.StdoutLogConsumer{}

	cRequest := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			//Tag:     "service_migrations_tag",
			Context: basePath,
		},
		ConfigModifier: func(config *container.Config) {
			config.Env = []string{
				"LOG_LEVEL=debug",
				"DO_MIGRATION=true",
				fmt.Sprintf("DATABASE_URL=%s", postgresqlUri),
			}
		},
		Networks:   networks,
		WaitingFor: wait.ForExit().WithExitTimeout(10 * time.Second),
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(2 * time.Second)},
			Consumers: []testcontainers.LogConsumer{&g},
		},
	}

	migrationC, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: cRequest,
			Started:          true,
		})
	if err != nil {
		return err
	}

	return migrationC.Terminate(ctx)
}
