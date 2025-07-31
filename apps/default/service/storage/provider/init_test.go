package provider

import (
	"context"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/local"
	"github.com/pitabwire/frame"
)

func TestGetStorageProvider(t *testing.T) {

	ctx := context.Background()

	cfg, err := frame.ConfigFromEnv[config.FilesConfig]()
	if err != nil {
		t.Errorf("Could not get file config : %v", err)
	}

	storageProvider, err := GetStorageProvider(ctx, &cfg)
	if err != nil {
		t.Errorf("A file storageProvider should has issues : %v", err)
	}

	_, ok := storageProvider.(*local.ProviderLocal)
	if !ok {
		t.Errorf("The storageProvider is supposed to be a local instance only")
	}

}
