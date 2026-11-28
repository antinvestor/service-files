package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/v3/datastore"
	"github.com/pitabwire/frame/v3/datastore/pool"
	"github.com/pitabwire/frame/v3/security"
	"github.com/pitabwire/frame/v3/tenancy"
	"github.com/pitabwire/frame/v3/workerpool"
	"gorm.io/gorm"
)

type MediaRepository interface {
	datastore.BaseRepository[*models.MediaMetadata]
	GetByHash(ctx context.Context, ownerId types.OwnerID, hash types.Base64Hash) (*models.MediaMetadata, error)
	GetByParentID(ctx context.Context, parentId types.MediaID) ([]*models.MediaMetadata, error)
	GetByParentIDAndThumbnailSize(ctx context.Context, parentId types.MediaID, thumbnailSize *types.ThumbnailSize) (*models.MediaMetadata, error)
	GetByOwnerID(ctx context.Context, ownerId types.OwnerID, query string, page int32, limit int32) ([]*models.MediaMetadata, error)
	// CreateWithObjectLock inserts media while holding ref's object lock.
	// ensureObject runs under the lock first and must leave the object in
	// place, so a concurrent ReleaseObject can never remove it between the
	// check and the insert.
	CreateWithObjectLock(ctx context.Context, media *models.MediaMetadata, ref ObjectRef, ensureObject func(context.Context) error) error
	// ReleaseObject, under ref's object lock and in one primary transaction:
	// soft-deletes row deleteID (when set; ErrRecordNotFound if absent),
	// then calls removeObject when no other live row in any tenant or
	// partition references the object. A removeObject error rolls the row
	// delete back.
	ReleaseObject(ctx context.Context, ref ObjectRef, deleteID string, removeObject func(context.Context) error) error
}

// ObjectRef identifies a stored object and the rows that may reference it.
type ObjectRef struct {
	// Bucket and Key are the resolved object location; they name the lock.
	Bucket string
	Key    string
	// ExcludeID is the row whose reference is being released.
	ExcludeID string
	// Legacy rows predate object_key; they reference the object through the
	// hash-derived key of their visibility bucket.
	Legacy bool
	Hash   types.Base64Hash
	Public bool
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

// objectLockNamespace is the first key of the two-key advisory lock, keeping
// object locks apart from any other advisory lock user of the database.
const objectLockNamespace = "service-files/media-object"

// lockObject takes the transaction-scoped advisory lock serialising every
// reference change to one stored object. It is released at commit/rollback,
// so it is correct behind a transaction-pooling PgBouncer as long as it is
// taken inside the same transaction as the statements it protects.
func lockObject(tx *gorm.DB, ref ObjectRef) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?), hashtext(?))",
		objectLockNamespace, ref.Bucket+"/"+ref.Key).Error
}

func (mr *mediaRepository) CreateWithObjectLock(ctx context.Context, media *models.MediaMetadata, ref ObjectRef, ensureObject func(context.Context) error) error {
	if media.GetVersion() > 0 {
		return errors.New("entity version is more than 0, consider using Update instead of Create")
	}
	return mr.Pool().DB(ctx, false).Transaction(func(tx *gorm.DB) error {
		if err := lockObject(tx, ref); err != nil {
			return fmt.Errorf("lock object: %w", err)
		}
		if err := ensureObject(ctx); err != nil {
			return err
		}
		return tx.Create(media).Error
	})
}

// releaseCtx elevates the release transaction to match-all visibility.
// Objects are shared by content across tenants and partitions, so the
// caller's row-level scope would hide references held elsewhere and let a
// delete remove an object other tenants still use. The row deleted in the
// transaction is one the caller has already read under its own scope and
// been authorised to delete; the count returns a number, never rows.
func releaseCtx(ctx context.Context) context.Context {
	return tenancy.WithSystemPrincipal(ctx, tenancy.SystemPrincipal{
		ServiceName: "service-files",
		Reason:      "media object reference release",
		AllowGlobal: true,
	})
}

func (mr *mediaRepository) ReleaseObject(ctx context.Context, ref ObjectRef, deleteID string, removeObject func(context.Context) error) error {
	return mr.Pool().DB(releaseCtx(ctx), false).Transaction(func(tx *gorm.DB) error {
		if err := lockObject(tx, ref); err != nil {
			return fmt.Errorf("lock object: %w", err)
		}

		if deleteID != "" {
			res := tx.Where("id = ?", deleteID).Delete(&models.MediaMetadata{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
		}

		refs := tx.Model(&models.MediaMetadata{}).Where("id <> ?", ref.ExcludeID)
		if ref.Legacy {
			refs = refs.Where("hash = ? AND public = ? AND COALESCE(object_key, '') = ''", string(ref.Hash), ref.Public)
		} else {
			refs = refs.Where("bucket_name = ? AND object_key = ?", ref.Bucket, ref.Key)
		}
		var n int64
		if err := refs.Count(&n).Error; err != nil {
			return fmt.Errorf("count object references: %w", err)
		}
		if n > 0 {
			return nil
		}
		return removeObject(ctx)
	})
}
