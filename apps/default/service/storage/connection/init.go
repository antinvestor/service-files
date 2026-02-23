package connection

import (
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/pitabwire/frame/workerpool"
)

func NewMediaDatabase(
	workManager workerpool.Manager,
	mediaRepo repository.MediaRepository,
	multipartUploadRepo repository.MultipartUploadRepository,
	multipartUploadPartRepo repository.MultipartUploadPartRepository,
	fileVersionRepo repository.FileVersionRepository,
	retentionPolicyRepo repository.RetentionPolicyRepository,
	fileRetentionRepo repository.FileRetentionRepository,
	storageStatsRepo repository.StorageStatsRepository,
) (storage.Database, error) {
	return &Database{
		WorkManager:             workManager,
		MediaRepository:         mediaRepo,
		MultipartUploadRepo:     multipartUploadRepo,
		MultipartUploadPartRepo: multipartUploadPartRepo,
		FileVersionRepo:         fileVersionRepo,
		RetentionPolicyRepo:     retentionPolicyRepo,
		FileRetentionRepo:       fileRetentionRepo,
		StorageStatsRepo:        storageStatsRepo,
	}, nil
}
