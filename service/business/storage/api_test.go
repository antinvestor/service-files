package storage

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetStorageProvider(t *testing.T) {

	ctx := context.Background()

	provider, err := GetStorageProvider(ctx, "LOCAL")
	assert.NoError(t, err, "A file provider should not have issues instantiating")

	assert.IsType(t, &ProviderLocal{}, provider, "The provider is supposed to be a local instance")

}

