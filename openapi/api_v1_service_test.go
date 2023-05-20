package openapi_test

import (
	"context"
	"fmt"
	"github.com/antinvestor/files/config"
	"github.com/antinvestor/files/openapi"
	"github.com/antinvestor/files/service/business/storage"
	"github.com/pitabwire/frame"
	"os"
	"testing"
)

func testService() (context.Context, *frame.Service, error) {

	dbURL := frame.GetEnv("TEST_DATABASE_URL",
		"postgres://ant:secret@localhost:5425/service_files?sslmode=disable")
	mainDB := frame.DatastoreCon(dbURL, false)

	var cfg config.FilesConfig
	err := frame.ConfigProcess("", &cfg)
	if err != nil {
		return nil, nil, err
	}

	fileQueuePublisher := frame.RegisterPublisher(cfg.QueueFileSyncName, cfg.QueueFileSyncURL)

	ctx, service := frame.NewService("file tests", frame.Config(&cfg), mainDB, fileQueuePublisher, frame.NoopDriver())
	_ = service.Run(ctx, "")
	return ctx, service, nil
}

func TestApiV1Service_AddFile(t *testing.T) {

	ctx, srv, err := testService()
	if err != nil {
		t.Errorf("Could not initialize service : %v", err)
		return
	}
	storageP, err := storage.GetStorageProvider(ctx, "LOCAL")
	if err != nil {
		t.Errorf("Could not get storage provider because : %v", err)
		return
	}

	path, _ := os.Getwd()
	fileName := fmt.Sprintf("%s/testing_data.txt", path)

	testFile, err := os.Create(fileName)
	if err != nil {
		t.Errorf("Could not create test file in : %v because : %v", path, err)
		return
	}

	_, err = testFile.WriteString("old\nfalcon\nsky\ncup\nforest\n")
	if err != nil {
		t.Errorf("Could not write to test file in : %v because : %v", fileName, err)
		return
	}

	apiService := openapi.NewApiV1Service(srv, storageP)
	response, err := apiService.AddFile(ctx, "access", "group", false, "testing.txt", testFile)
	if err != nil {
		t.Errorf("Could not add file because : %v", err)
		return
	}

	f, ok := response.Body.(openapi.File)
	if !ok {
		t.Errorf("response body is not instance of file")
	}

	if f.Name != "testing.txt" {
		t.Error("The file names don't match")
	}

}
