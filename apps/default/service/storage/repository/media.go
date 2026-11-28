package repository

import (
	"context"
	"strconv"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/v3/datastore"
	"github.com/pitabwire/frame/v3/datastore/pool"
	"github.com/pitabwire/frame/v3/security"
	"github.com/pitabwire/frame/v3/tenancy"
	"github.com/pitabwire/frame/v3/workerpool"
)

type MediaRepository interface {
	datastore.BaseRepository[*models.MediaMetadata]
	GetByHash(ctx context.Context, ownerId types.OwnerID, hash types.Base64Hash) (*models.MediaMetadata, error)
	GetByParentID(ctx context.Context, parentId types.MediaID) ([]*models.MediaMetadata, error)
	GetByParentIDAndThumbnailSize(ctx context.Context, parentId types.MediaID, thumbnailSize *types.ThumbnailSize) (*models.MediaMetadata, error)
	GetByOwnerID(ctx context.Context, ownerId types.OwnerID, query string, page int32, limit int32) ([]*models.MediaMetadata, error)
	// CountObjectReferences counts live rows other than excludeID stored
	// under bucketName/objectKey, across every tenant and partition.
	CountObjectReferences(ctx context.Context, excludeID, bucketName, objectKey string) (int64, error)
	// CountLegacyObjectReferences counts live rows other than excludeID that
	// predate object_key and resolve to the same legacy hash-derived object,
	// across every tenant and partition.
	CountLegacyObjectReferences(ctx context.Context, excludeID string, hash types.Base64Hash, public bool) (int64, error)
}

func NewMediaRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) MediaRepository {
	fileRepo := mediaRepository{
		BaseRepository: datastore.NewBaseRepository[*models.MediaMetadata](
			ctx, dbPool, workMan, func() *models.MediaMetadata { return &models.MediaMetadata{} },
		),
	}
	fileRepo.ExtendFieldsAllowed("search_properties")
	return &fileRepo
}

type mediaRepository struct {
	datastore.BaseRepository[*models.MediaMetadata]
}

func (mr *mediaRepository) GetByHash(ctx context.Context, ownerId types.OwnerID, hash types.Base64Hash) (*models.MediaMetadata, error) {
	// Scope dedupe to the partition the new upload would land in (the
	// caller's claims, as GenID uses). Internal callers bypass row-level
	// tenancy, so without this a hash match could return another
	// partition's media.
	var tenantID, partitionID string
	if claims := security.ClaimsFromContext(ctx); claims != nil {
		tenantID, partitionID = claims.GetTenantID(), claims.GetPartitionID()
	}

	file := &models.MediaMetadata{}
	err := mr.Pool().DB(ctx, true).First(file,
		"owner_id = ? AND hash = ? AND tenant_id = ? AND partition_id = ?",
		string(ownerId), string(hash), tenantID, partitionID).Error
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (mr *mediaRepository) GetByParentID(ctx context.Context, parentId types.MediaID) ([]*models.MediaMetadata, error) {
	var media []*models.MediaMetadata
	err := mr.Pool().DB(ctx, true).Where("parent_id = ?", string(parentId)).Find(&media).Error
	if err != nil {
		return nil, err
	}

	return media, nil
}

func (mr *mediaRepository) GetByParentIDAndThumbnailSize(ctx context.Context, parentId types.MediaID, thumbnailSize *types.ThumbnailSize) (*models.MediaMetadata, error) {
	media := &models.MediaMetadata{}
	tx := mr.Pool().DB(ctx, true).Where(" parent_id = ? ", string(parentId))
	if thumbnailSize != nil {
		tx = tx.Where("properties ->> 'h' = ?  "+
			"AND properties ->> 'w' = ?  "+
			"AND properties ->> 'm' = ? ",
			strconv.Itoa(thumbnailSize.Height), strconv.Itoa(thumbnailSize.Width), thumbnailSize.ResizeMethod)
	}

	err := tx.First(media).Error
	if err != nil {
		return nil, err
	}

	return media, nil
}

func (mr *mediaRepository) GetByOwnerID(ctx context.Context, ownerId types.OwnerID, query string, page int32, limit int32) ([]*models.MediaMetadata, error) {
	fileList := make([]*models.MediaMetadata, 0)
	tx := mr.Pool().DB(ctx, true).Where(" owner_id = ? ", string(ownerId))

	if query != "" {
		tx = tx.Where("name LIKE ?", "%"+query+"%")
	}

	// Use 0-based pagination: page 0 is the first page
	offset := page * limit
	tx = tx.Offset(int(offset))
	tx = tx.Limit(int(limit))

	err := tx.Find(&fileList).Error
	if err != nil {
		return nil, err
	}

	return fileList, nil
}

// Objects are shared by content across tenants and partitions, so a caller's
// row-level tenancy scope would hide references held elsewhere and let a
// delete remove an object other tenants still use. The counts therefore run
// with tenancy enforcement skipped; they return only a number, never rows.

func (mr *mediaRepository) CountObjectReferences(ctx context.Context, excludeID, bucketName, objectKey string) (int64, error) {
	var n int64
	err := mr.Pool().DB(tenancy.WithSkipEnforcement(ctx), true).
		Model(&models.MediaMetadata{}).
		Where("id <> ? AND bucket_name = ? AND object_key = ?", excludeID, bucketName, objectKey).
		Count(&n).Error
	return n, err
}

func (mr *mediaRepository) CountLegacyObjectReferences(ctx context.Context, excludeID string, hash types.Base64Hash, public bool) (int64, error) {
	var n int64
	err := mr.Pool().DB(tenancy.WithSkipEnforcement(ctx), true).
		Model(&models.MediaMetadata{}).
		Where("id <> ? AND hash = ? AND public = ? AND COALESCE(object_key, '') = ''", excludeID, string(hash), public).
		Count(&n).Error
	return n, err
}
