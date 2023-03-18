package openapi

import (
	"context"
	"github.com/antinvestor/files/config"
	"github.com/antinvestor/files/service/business/storage"
	"github.com/pitabwire/frame"
	"os"
	"testing"
)

func testService(ctx context.Context) (*frame.Service, error) {

	dbURL := frame.GetEnv("TEST_DATABASE_URL",
		"postgres://ant:secret@localhost:5425/service_files?sslmode=disable")
	mainDB := frame.DatastoreCon(ctx, dbURL, false)

	var cfg config.FilesConfig
	err := frame.ConfigProcess("", &cfg)
	if err != nil {
		return nil, err
	}

	fileQueuePublisher := frame.RegisterPublisher(cfg.QueueFileSyncName, cfg.QueueFileSyncURL)

	service := frame.NewService("file tests", frame.Config(&cfg), mainDB, fileQueuePublisher, frame.NoopDriver())
	_ = service.Run(ctx, "")
	return service, nil
}

func TestApiV1Service_AddFile(t *testing.T) {

	ctx := context.Background()
	srv, err := testService(ctx)
	if err != nil {
		t.Errorf("Could not initialize service : %v", err)
		return
	}
	storageP, err := storage.GetStorageProvider(ctx, "LOCAL")
	if err != nil {
		t.Errorf("Could not get storage provider because : %v", err)
		return
	}

	testFile, err := os.Open("../tests_runner/sample3.txt")
	if err != nil {
		path, _ := os.Getwd()
		t.Errorf("Could not read test file in : %v because : %v", path, err)
		return
	}

	apiService := NewApiV1Service(srv, storageP)
	response, err := apiService.AddFile(ctx, "access", "group", false, "testing.txt", testFile)
	if err != nil {
		t.Errorf("Could not add file because : %v", err)
		return
	}

	f, ok := response.Body.(File)
	if !ok {
		t.Errorf("response body is not instance of file")
	}

	if f.Name != "testing.txt" {
		t.Error("The file names don't match")
	}

}
