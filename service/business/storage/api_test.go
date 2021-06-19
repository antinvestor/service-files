package storage

import (
	"context"
	"testing"
)

func TestGetStorageProvider(t *testing.T) {

	ctx := context.Background()

	provider, err := GetStorageProvider(ctx, "LOCAL")
	if err != nil {
		t.Errorf("A file provider should has issues : %v", err)
	}

	_, ok := provider.(*ProviderLocal)
	if !ok {
		t.Errorf("The provider is supposed to be a local instance only")
	}

}
