package repository

import (
	"context"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

// RetentionPolicyRepository defines the interface for retention policy operations
type RetentionPolicyRepository interface {
	datastore.BaseRepository[*models.RetentionPolicy]
	GetByID(ctx context.Context, policyID string) (*models.RetentionPolicy, error)
	GetDefault(ctx context.Context) (*models.RetentionPolicy, error)
	ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*models.RetentionPolicy, int, error)
}

// NewRetentionPolicyRepository creates a new retention policy repository instance
func NewRetentionPolicyRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) RetentionPolicyRepository {
	repo := retentionPolicyRepository{
		BaseRepository: datastore.NewBaseRepository[*models.RetentionPolicy](
			ctx, dbPool, workMan, func() *models.RetentionPolicy { return &models.RetentionPolicy{} },
		),
	}
	return &repo
}

type retentionPolicyRepository struct {
	datastore.BaseRepository[*models.RetentionPolicy]
}

// GetByID retrieves a retention policy by its ID
func (r *retentionPolicyRepository) GetByID(ctx context.Context, policyID string) (*models.RetentionPolicy, error) {
	policy := &models.RetentionPolicy{}
	err := r.Pool().DB(ctx, true).First(policy, "id = ?", policyID).Error
	if err != nil {
		return nil, nil
	}
	return policy, nil
}

// GetDefault retrieves the default retention policy
func (r *retentionPolicyRepository) GetDefault(ctx context.Context) (*models.RetentionPolicy, error) {
	policy := &models.RetentionPolicy{}
	err := r.Pool().DB(ctx, true).Where("is_default = ?", true).First(policy).Error
	if err != nil {
		return nil, err
	}
	return policy, nil
}

// ListByOwner retrieves policies owned by a given owner with pagination
func (r *retentionPolicyRepository) ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*models.RetentionPolicy, int, error) {
	var policies []*models.RetentionPolicy
	var count int64

	query := r.Pool().DB(ctx, true).Where("owner_id = ?", ownerID)

	// Get total count
	if err := query.Model(&models.RetentionPolicy{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&policies).Error
	if err != nil {
		return nil, 0, err
	}

	return policies, int(count), nil
}
