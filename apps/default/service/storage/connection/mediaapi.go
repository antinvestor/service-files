// Copyright 2022 The Matrix.org Foundation C.I.C.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connection

import (
	"context"
	"errors"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/workerpool"
	"github.com/pitabwire/util"
	"gorm.io/gorm"
)

type Database struct {
	WorkManager             workerpool.Manager
	MediaRepository         repository.MediaRepository
	MultipartUploadRepo     repository.MultipartUploadRepository
	MultipartUploadPartRepo repository.MultipartUploadPartRepository
	FileVersionRepo         repository.FileVersionRepository
	RetentionPolicyRepo     repository.RetentionPolicyRepository
	FileRetentionRepo       repository.FileRetentionRepository
	StorageStatsRepo        repository.StorageStatsRepository
}

// StoreMediaMetadata inserts the metadata about the uploaded media into the database.
// Returns an error if the combination of MediaID and Origin are not unique in the table.
func (d *Database) StoreMediaMetadata(ctx context.Context, mediaMetadata *types.MediaMetadata) error {
	media := models.MediaMetadata{}
	media.Fill(mediaMetadata)
	return d.MediaRepository.Create(ctx, &media)
}

// GetMediaMetadata returns metadata about media stored on this server.
// The media could have been uploaded to this server or fetched from another server and cached here.
// Returns nil metadata if there is no metadata associated with this media.
func (d *Database) GetMediaMetadata(ctx context.Context, mediaID types.MediaID) (*types.MediaMetadata, error) {
	mediaMetadata, err := d.MediaRepository.GetByID(ctx, string(mediaID))
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return mediaMetadata.ToApi(), err
}

// GetMediaMetadataByHash returns metadata about media stored on this server.
// The media could have been uploaded to this server or fetched from another server and cached here.
// Returns nil metadata if there is no metadata associated with this media.
func (d *Database) GetMediaMetadataByHash(ctx context.Context, ownerId types.OwnerID, mediaHash types.Base64Hash) (*types.MediaMetadata, error) {
	mediaMetadata, err := d.MediaRepository.GetByHash(ctx, ownerId, mediaHash)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return mediaMetadata.ToApi(), err
}

func (d *Database) Search(ctx context.Context, query *data.SearchQuery) (workerpool.JobResultPipe[*types.MediaMetadata], error) {

	jobResult := workerpool.NewJob(func(ctx context.Context, result workerpool.JobResultPipe[*types.MediaMetadata]) error {

		metadataResult, err := d.MediaRepository.Search(ctx, query)

		if err != nil {
			return err
		}

		for {

			res, ok := metadataResult.ReadResult(ctx)
			if !ok {
				return nil
			}

			if res.IsError() {
				return res.Error()
			}

			for _, mediaMetadata := range res.Item() {
				err = result.WriteResult(ctx, mediaMetadata.ToApi())
				if err != nil {
					return err
				}
			}
		}
	})

	err := workerpool.SubmitJob(ctx, d.WorkManager, jobResult)
	if err != nil {
		return nil, err
	}

	return jobResult, nil
}

// DeleteMedia deletes media from the database
func (d *Database) DeleteMedia(ctx context.Context, mediaID types.MediaID) error {
	return d.MediaRepository.Delete(ctx, string(mediaID))
}

// GetUserUsage returns the total storage used by a user and file count
func (d *Database) GetUserUsage(ctx context.Context, ownerID types.OwnerID) (int64, int, error) {
	type result struct {
		TotalSize int64
		FileCount int
	}
	var r result

	err := d.MediaRepository.Pool().DB(ctx, true).Model(&models.MediaMetadata{}).
		Select("COALESCE(SUM(size), 0) as total_size, COUNT(*) as file_count").
		Where("owner_id = ?", string(ownerID)).
		Scan(&r).Error

	if err != nil {
		return 0, 0, err
	}
	return r.TotalSize, r.FileCount, nil
}

// StoreThumbnail inserts the metadata about the thumbnail into the database.
// Returns an error if the combination of MediaID and Origin are not unique in the table.
func (d *Database) StoreThumbnail(ctx context.Context, thumbnailMetadata *types.ThumbnailMetadata) error {
	return d.StoreMediaMetadata(ctx, thumbnailMetadata.MediaMetadata)
}

// GetThumbnail returns metadata about a specific thumbnail.
// The media could have been uploaded to this server or fetched from another server and cached here.
// Returns nil metadata if there is no metadata associated with this thumbnail.
func (d *Database) GetThumbnail(ctx context.Context, mediaID types.MediaID, width, height int, resizeMethod string) (*types.ThumbnailMetadata, error) {

	thumbnailSize := types.ThumbnailSize{
		Width:        width,
		Height:       height,
		ResizeMethod: resizeMethod,
	}
	mediaMetadata, err := d.MediaRepository.GetByParentIDAndThumbnailSize(ctx, mediaID, &thumbnailSize)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	apiMM := mediaMetadata.ToApi()

	return &types.ThumbnailMetadata{
		MediaMetadata: apiMM,
	}, nil
}

// GetThumbnails returns metadata about all thumbnails for a specific media stored on this server.
// The media could have been uploaded to this server or fetched from another server and cached here.
// Returns nil metadata if there are no thumbnails associated with this media.
func (d *Database) GetThumbnails(ctx context.Context, mediaID types.MediaID) ([]*types.ThumbnailMetadata, error) {
	metadatas, err := d.MediaRepository.GetByParentID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	apiTMM := make([]*types.ThumbnailMetadata, len(metadatas))
	for i, mm := range metadatas {
		apiTMM[i] = &types.ThumbnailMetadata{
			MediaMetadata: mm.ToApi(),
		}
	}

	return apiTMM, err
}

func (d *Database) StoreUpload(ctx context.Context, upload interface {
	GetID() string
	GetOwnerID() string
	GetMediaID() string
	GetUploadName() string
	GetContentType() string
	GetTotalSize() int64
	GetPartSize() int64
	GetPartCount() int
	GetUploadState() string
	GetExpiresAt() *time.Time
}) error {
	m := &models.MultipartUpload{
		BaseModel:   data.BaseModel{ID: upload.GetID()},
		OwnerID:     upload.GetOwnerID(),
		MediaID:     upload.GetMediaID(),
		UploadName:  upload.GetUploadName(),
		ContentType: upload.GetContentType(),
		TotalSize:   upload.GetTotalSize(),
		PartSize:    upload.GetPartSize(),
		PartCount:   upload.GetPartCount(),
		UploadState: upload.GetUploadState(),
		ExpiresAt:   upload.GetExpiresAt(),
	}
	return d.MultipartUploadRepo.Create(ctx, m)
}

type dbMultipartUploadResult struct {
	m *models.MultipartUpload
}

func (m *dbMultipartUploadResult) ID() string            { return m.m.ID }
func (m *dbMultipartUploadResult) OwnerID() string       { return m.m.OwnerID }
func (m *dbMultipartUploadResult) MediaID() string       { return m.m.MediaID }
func (m *dbMultipartUploadResult) UploadName() string    { return m.m.UploadName }
func (m *dbMultipartUploadResult) ContentType() string   { return m.m.ContentType }
func (m *dbMultipartUploadResult) TotalSize() int64      { return m.m.TotalSize }
func (m *dbMultipartUploadResult) PartSize() int64       { return m.m.PartSize }
func (m *dbMultipartUploadResult) PartCount() int        { return m.m.PartCount }
func (m *dbMultipartUploadResult) UploadedParts() int    { return m.m.UploadedParts }
func (m *dbMultipartUploadResult) UploadState() string   { return m.m.UploadState }
func (m *dbMultipartUploadResult) ExpiresAt() *time.Time { return m.m.ExpiresAt }

func (d *Database) GetUpload(ctx context.Context, uploadID string) (interface {
	ID() string
	OwnerID() string
	MediaID() string
	UploadName() string
	ContentType() string
	TotalSize() int64
	PartSize() int64
	PartCount() int
	UploadedParts() int
	UploadState() string
	ExpiresAt() *time.Time
}, error) {
	m, err := d.MultipartUploadRepo.GetByUploadID(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	return &dbMultipartUploadResult{m: m}, nil
}

func (d *Database) UpdateUploadState(ctx context.Context, uploadID string, state string) error {
	return d.MultipartUploadRepo.UpdateState(ctx, uploadID, state)
}

func (d *Database) DeleteUpload(ctx context.Context, uploadID string) error {
	return d.MultipartUploadRepo.Delete(ctx, uploadID)
}

func (d *Database) StorePart(ctx context.Context, part interface {
	GetID() string
	GetUploadID() string
	GetPartNumber() int
	GetEtag() string
	GetSize() int64
	GetContentHash() string
	GetStoragePath() string
}) error {
	p := &models.MultipartUploadPart{
		BaseModel:   data.BaseModel{ID: part.GetID()},
		UploadID:    part.GetUploadID(),
		PartNumber:  part.GetPartNumber(),
		Etag:        part.GetEtag(),
		Size:        part.GetSize(),
		ContentHash: part.GetContentHash(),
		StoragePath: part.GetStoragePath(),
		IsUploaded:  true,
	}
	return d.MultipartUploadPartRepo.Create(ctx, p)
}

type dbMultipartUploadPartResult struct {
	p *models.MultipartUploadPart
}

func (p *dbMultipartUploadPartResult) ID() string          { return p.p.ID }
func (p *dbMultipartUploadPartResult) UploadID() string    { return p.p.UploadID }
func (p *dbMultipartUploadPartResult) PartNumber() int     { return p.p.PartNumber }
func (p *dbMultipartUploadPartResult) Etag() string        { return p.p.Etag }
func (p *dbMultipartUploadPartResult) Size() int64         { return p.p.Size }
func (p *dbMultipartUploadPartResult) ContentHash() string { return p.p.ContentHash }
func (p *dbMultipartUploadPartResult) StoragePath() string { return p.p.StoragePath }

func (d *Database) GetParts(ctx context.Context, uploadID string) ([]interface {
	ID() string
	UploadID() string
	PartNumber() int
	Etag() string
	Size() int64
	ContentHash() string
	StoragePath() string
}, error) {
	parts, err := d.MultipartUploadPartRepo.GetByUploadID(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	result := make([]interface {
		ID() string
		UploadID() string
		PartNumber() int
		Etag() string
		Size() int64
		ContentHash() string
		StoragePath() string
	}, len(parts))
	for i, p := range parts {
		result[i] = &dbMultipartUploadPartResult{p: p}
	}
	return result, nil
}

func (d *Database) GetPart(ctx context.Context, uploadID string, partNumber int) (interface {
	ID() string
	UploadID() string
	PartNumber() int
	Etag() string
	Size() int64
	ContentHash() string
	StoragePath() string
}, error) {
	p, err := d.MultipartUploadPartRepo.GetPart(ctx, uploadID, partNumber)
	if err != nil {
		return nil, err
	}
	return &dbMultipartUploadPartResult{p: p}, nil
}

type dbFileVersionResult struct {
	v *models.FileVersion
}

func (v *dbFileVersionResult) ID() string           { return v.v.ID }
func (v *dbFileVersionResult) MediaID() string      { return v.v.MediaID }
func (v *dbFileVersionResult) VersionNumber() int   { return v.v.VersionNumber }
func (v *dbFileVersionResult) ContentHash() string  { return v.v.ContentHash }
func (v *dbFileVersionResult) FileSize() int64      { return v.v.FileSize }
func (v *dbFileVersionResult) UploadName() string   { return v.v.UploadName }
func (v *dbFileVersionResult) ContentType() string  { return v.v.ContentType }
func (v *dbFileVersionResult) CreatedAt() time.Time { return v.v.CreatedAt }
func (v *dbFileVersionResult) StoragePath() string  { return v.v.StoragePath }

func (d *Database) CreateVersion(ctx context.Context, version interface {
	GetMediaID() string
	GetVersionNumber() int
	GetContentHash() string
	GetFileSize() int64
	GetUploadName() string
	GetContentType() string
	GetStoragePath() string
	GetCreatedBy() string
}) error {
	v := &models.FileVersion{
		MediaID:       version.GetMediaID(),
		VersionNumber: version.GetVersionNumber(),
		ContentHash:   version.GetContentHash(),
		FileSize:      version.GetFileSize(),
		UploadName:    version.GetUploadName(),
		ContentType:   version.GetContentType(),
		StoragePath:   version.GetStoragePath(),
		CreatedBy:     version.GetCreatedBy(),
	}
	v.ID = util.IDString()
	return d.FileVersionRepo.Create(ctx, v)
}

func (d *Database) GetVersions(ctx context.Context, mediaID string) ([]interface {
	ID() string
	MediaID() string
	VersionNumber() int
	ContentHash() string
	FileSize() int64
	UploadName() string
	ContentType() string
	CreatedAt() time.Time
}, error) {
	versions, err := d.FileVersionRepo.GetByMediaID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	result := make([]interface {
		ID() string
		MediaID() string
		VersionNumber() int
		ContentHash() string
		FileSize() int64
		UploadName() string
		ContentType() string
		CreatedAt() time.Time
	}, len(versions))
	for i, v := range versions {
		result[i] = &dbFileVersionResult{v: v}
	}
	return result, nil
}

func (d *Database) GetVersion(ctx context.Context, mediaID string, versionNumber int) (interface {
	ID() string
	MediaID() string
	VersionNumber() int
	ContentHash() string
	FileSize() int64
	UploadName() string
	ContentType() string
	StoragePath() string
	CreatedAt() time.Time
}, error) {
	v, err := d.FileVersionRepo.GetVersion(ctx, mediaID, versionNumber)
	if err != nil {
		return nil, err
	}
	return &dbFileVersionResult{v: v}, nil
}

func (d *Database) GetVersionsPaginated(ctx context.Context, mediaID string, limit, offset int) ([]interface {
	ID() string
	MediaID() string
	VersionNumber() int
	ContentHash() string
	FileSize() int64
	UploadName() string
	ContentType() string
	CreatedAt() time.Time
}, int, error) {
	versions, count, err := d.FileVersionRepo.GetVersionsPaginated(ctx, mediaID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	result := make([]interface {
		ID() string
		MediaID() string
		VersionNumber() int
		ContentHash() string
		FileSize() int64
		UploadName() string
		ContentType() string
		CreatedAt() time.Time
	}, len(versions))
	for i, v := range versions {
		result[i] = &dbFileVersionResult{v: v}
	}
	return result, count, nil
}

func (d *Database) RestoreMediaToVersion(ctx context.Context, mediaID string, versionNumber int, restoredBy string) (*types.MediaMetadata, error) {
	var restored *types.MediaMetadata
	err := d.MediaRepository.Pool().DB(ctx, false).Transaction(func(tx *gorm.DB) error {
		version := &models.FileVersion{}
		if err := tx.Where("media_id = ? AND version_number = ?", mediaID, versionNumber).First(version).Error; err != nil {
			return err
		}

		media := &models.MediaMetadata{}
		if err := tx.First(media, "id = ?", mediaID).Error; err != nil {
			return err
		}

		var maxVersion int64
		if err := tx.Model(&models.FileVersion{}).
			Where("media_id = ?", mediaID).
			Select("COALESCE(MAX(version_number), 0)").
			Scan(&maxVersion).Error; err != nil {
			return err
		}

		backup := &models.FileVersion{
			BaseModel:          data.BaseModel{ID: util.IDString()},
			MediaID:            mediaID,
			VersionNumber:      int(maxVersion) + 1,
			ContentHash:        media.Hash,
			FileSize:           media.Size,
			UploadName:         media.Name,
			ContentType:        media.Mimetype,
			StoragePath:        media.Hash,
			CreatedBy:          restoredBy,
			RestoreFromVersion: version.VersionNumber,
		}
		if err := tx.Create(backup).Error; err != nil {
			return err
		}

		media.Hash = version.ContentHash
		media.Size = version.FileSize
		media.Name = version.UploadName
		media.Mimetype = version.ContentType
		media.OriginTs = time.Now().UnixMilli()

		if err := tx.Save(media).Error; err != nil {
			return err
		}

		restored = media.ToApi()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return restored, nil
}

type dbRetentionPolicyResult struct {
	p *models.RetentionPolicy
}

func (p *dbRetentionPolicyResult) ID() string          { return p.p.ID }
func (p *dbRetentionPolicyResult) Name() string        { return p.p.Name }
func (p *dbRetentionPolicyResult) Description() string { return p.p.Description }
func (p *dbRetentionPolicyResult) RetentionDays() int  { return p.p.RetentionDays }
func (p *dbRetentionPolicyResult) IsDefault() bool     { return p.p.IsDefault }
func (p *dbRetentionPolicyResult) IsSystem() bool      { return p.p.IsSystem }
func (p *dbRetentionPolicyResult) OwnerID() string     { return p.p.OwnerID }

func (d *Database) CreatePolicy(ctx context.Context, policy interface {
	GetID() string
	GetName() string
	GetDescription() string
	GetRetentionDays() int
	GetIsDefault() bool
	GetIsSystem() bool
	GetOwnerID() string
}) error {
	p := &models.RetentionPolicy{
		BaseModel:     data.BaseModel{ID: policy.GetID()},
		Name:          policy.GetName(),
		Description:   policy.GetDescription(),
		RetentionDays: policy.GetRetentionDays(),
		IsDefault:     policy.GetIsDefault(),
		IsSystem:      policy.GetIsSystem(),
		OwnerID:       policy.GetOwnerID(),
	}
	return d.RetentionPolicyRepo.Create(ctx, p)
}

func (d *Database) GetPolicy(ctx context.Context, policyID string) (interface {
	ID() string
	Name() string
	Description() string
	RetentionDays() int
	IsDefault() bool
	IsSystem() bool
	OwnerID() string
}, error) {
	p, err := d.RetentionPolicyRepo.GetByID(ctx, policyID)
	if err != nil {
		return nil, err
	}
	return &dbRetentionPolicyResult{p: p}, nil
}

func (d *Database) UpdatePolicy(ctx context.Context, policy interface {
	GetID() string
	GetName() string
	GetDescription() string
	GetRetentionDays() int
	GetIsDefault() bool
}) error {
	return d.RetentionPolicyRepo.Pool().DB(ctx, true).Model(&models.RetentionPolicy{}).
		Where("id = ?", policy.GetID()).
		Updates(map[string]any{
			"name":           policy.GetName(),
			"description":    policy.GetDescription(),
			"retention_days": policy.GetRetentionDays(),
			"is_default":     policy.GetIsDefault(),
		}).Error
}

func (d *Database) DeletePolicy(ctx context.Context, policyID string) error {
	return d.RetentionPolicyRepo.Delete(ctx, policyID)
}

func (d *Database) ListPolicies(ctx context.Context, ownerID string, limit, offset int) ([]interface {
	ID() string
	Name() string
	Description() string
	RetentionDays() int
	IsDefault() bool
	IsSystem() bool
	OwnerID() string
}, int, error) {
	policies, count, err := d.RetentionPolicyRepo.ListByOwner(ctx, ownerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	result := make([]interface {
		ID() string
		Name() string
		Description() string
		RetentionDays() int
		IsDefault() bool
		IsSystem() bool
		OwnerID() string
	}, len(policies))
	for i, p := range policies {
		result[i] = &dbRetentionPolicyResult{p: p}
	}
	return result, count, nil
}

type dbFileRetentionResult struct {
	r *models.FileRetention
}

func (r *dbFileRetentionResult) MediaID() string       { return r.r.MediaID }
func (r *dbFileRetentionResult) PolicyID() string      { return r.r.PolicyID }
func (r *dbFileRetentionResult) AppliedAt() time.Time  { return r.r.AppliedAt }
func (r *dbFileRetentionResult) ExpiresAt() *time.Time { return r.r.ExpiresAt }
func (r *dbFileRetentionResult) IsLocked() bool        { return r.r.IsLocked }

func (d *Database) ApplyRetention(ctx context.Context, retention interface {
	GetMediaID() string
	GetPolicyID() string
	GetExpiresAt() *time.Time
	GetIsLocked() bool
}) error {
	r := &models.FileRetention{
		MediaID:   retention.GetMediaID(),
		PolicyID:  retention.GetPolicyID(),
		AppliedAt: time.Now(),
		ExpiresAt: retention.GetExpiresAt(),
		IsLocked:  retention.GetIsLocked(),
	}
	r.ID = util.IDString()
	return d.FileRetentionRepo.Create(ctx, r)
}

func (d *Database) GetRetention(ctx context.Context, mediaID string) (interface {
	MediaID() string
	PolicyID() string
	AppliedAt() time.Time
	ExpiresAt() *time.Time
	IsLocked() bool
}, error) {
	r, err := d.FileRetentionRepo.GetByMediaID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	return &dbFileRetentionResult{r: r}, nil
}

func (d *Database) RemoveRetention(ctx context.Context, mediaID string) error {
	return d.FileRetentionRepo.DeleteByMediaID(ctx, mediaID)
}

type dbFileRetentionExpired struct {
	r *models.FileRetention
}

func (r *dbFileRetentionExpired) MediaID() string       { return r.r.MediaID }
func (r *dbFileRetentionExpired) PolicyID() string      { return r.r.PolicyID }
func (r *dbFileRetentionExpired) ExpiresAt() *time.Time { return r.r.ExpiresAt }

func (d *Database) GetExpiredRetentions(ctx context.Context, before time.Time) ([]interface {
	MediaID() string
	PolicyID() string
	ExpiresAt() *time.Time
}, error) {
	retentions, err := d.FileRetentionRepo.GetExpired(ctx, before)
	if err != nil {
		return nil, err
	}
	result := make([]interface {
		MediaID() string
		PolicyID() string
		ExpiresAt() *time.Time
	}, len(retentions))
	for i, r := range retentions {
		result[i] = &dbFileRetentionExpired{r: r}
	}
	return result, nil
}

func (d *Database) LockRetention(ctx context.Context, mediaID string, locked bool) error {
	return d.FileRetentionRepo.Pool().DB(ctx, true).Model(&models.FileRetention{}).
		Where("media_id = ?", mediaID).
		Update("is_locked", locked).Error
}

type dbStorageStatsResult struct {
	s *models.StorageStats
}

func (s *dbStorageStatsResult) TotalBytes() int64   { return s.s.TotalBytes }
func (s *dbStorageStatsResult) FileCount() int      { return s.s.FileCount }
func (s *dbStorageStatsResult) UserCount() int      { return s.s.UserCount }
func (s *dbStorageStatsResult) PublicBytes() int64  { return s.s.PublicBytes }
func (s *dbStorageStatsResult) PrivateBytes() int64 { return s.s.PrivateBytes }

func (d *Database) RecordStats(ctx context.Context, stats interface {
	GetTotalBytes() int64
	GetFileCount() int
	GetUserCount() int
	GetPublicBytes() int64
	GetPrivateBytes() int64
}) error {
	s := &models.StorageStats{
		BaseModel:    data.BaseModel{ID: util.IDString()},
		RecordDate:   time.Now().Truncate(24 * time.Hour),
		TotalBytes:   stats.GetTotalBytes(),
		FileCount:    stats.GetFileCount(),
		UserCount:    stats.GetUserCount(),
		PublicBytes:  stats.GetPublicBytes(),
		PrivateBytes: stats.GetPrivateBytes(),
	}
	return d.StorageStatsRepo.Create(ctx, s)
}

func (d *Database) GetStats(ctx context.Context, date time.Time) (interface {
	TotalBytes() int64
	FileCount() int
	UserCount() int
	PublicBytes() int64
	PrivateBytes() int64
}, error) {
	s, err := d.StorageStatsRepo.GetByDate(ctx, date)
	if err != nil {
		return nil, err
	}
	return &dbStorageStatsResult{s: s}, nil
}

type dbStorageStatsRange struct {
	s *models.StorageStats
}

func (s *dbStorageStatsRange) Date() time.Time   { return s.s.RecordDate }
func (s *dbStorageStatsRange) TotalBytes() int64 { return s.s.TotalBytes }
func (s *dbStorageStatsRange) FileCount() int    { return s.s.FileCount }
func (s *dbStorageStatsRange) UserCount() int    { return s.s.UserCount }

func (d *Database) GetStatsRange(ctx context.Context, start, end time.Time) ([]interface {
	Date() time.Time
	TotalBytes() int64
	FileCount() int
	UserCount() int
}, error) {
	stats, err := d.StorageStatsRepo.GetRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	result := make([]interface {
		Date() time.Time
		TotalBytes() int64
		FileCount() int
		UserCount() int
	}, len(stats))
	for i, s := range stats {
		result[i] = &dbStorageStatsRange{s: s}
	}
	return result, nil
}

func (d *Database) GetLatestStats(ctx context.Context) (interface {
	TotalBytes() int64
	FileCount() int
	UserCount() int
	PublicBytes() int64
	PrivateBytes() int64
}, error) {
	s, err := d.StorageStatsRepo.GetLatest(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &dbStorageStatsResult{s: s}, nil
}
