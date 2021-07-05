package openapi

import (
	"context"
	"fmt"
	"github.com/antinvestor/files/config"
	"github.com/antinvestor/files/service/business/storage"
	"github.com/pitabwire/frame"
	"os"
	"testing"
)

const testDatastoreConnection = "postgres://file:secret@localhost:5425/filedatabase?sslmode=disable"

func testService(ctx context.Context) *frame.Service {

	dbUrl := frame.GetEnv(fmt.Sprintf("%s_TEST", config.EnvDatabaseUrl), testDatastoreConnection)
	mainDb := frame.Datastore(ctx, dbUrl, false)

	fileQueueURL := fmt.Sprintf("mem://%s", config.QueueFileSyncName)
	fileQueuePublisher := frame.RegisterPublisher(config.QueueFileSyncName, fileQueueURL)

	service := frame.NewService("file tests", mainDb, fileQueuePublisher, frame.NoopHttpOptions())
	_ = service.Run(ctx, "")
	return service
}


func TestApiV1Service_AddFile(t *testing.T) {

	ctx := context.Background()
	srv := testService(ctx)
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
	if !ok{
		t.Errorf("response body is not instance of file")
	}

	if f.Name != "testing.txt" {
		t.Error("The file names don't match")
	}

}

