package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/antinvestor/service-files/apps/property/service/models"
	"github.com/pitabwire/frame/datastore/pool"
	"gorm.io/gorm"
)

type SubscriptionRepository interface {
	GetByID(ctx context.Context, id string) (*models.Subscription, error)
	GetByPropertyID(ctx context.Context, propertyId string, query string) ([]models.Subscription, error)
	Save(ctx context.Context, subscription *models.Subscription) error
	Delete(ctx context.Context, id string) error
}

type subscriptionRepository struct {
	dbPool pool.Pool
}

func NewSubscriptionRepository(dbPool pool.Pool) SubscriptionRepository {
	return &subscriptionRepository{dbPool: dbPool}
}

func (repo *subscriptionRepository) GetByID(ctx context.Context, id string) (*models.Subscription, error) {
	subscription := models.Subscription{}
	err := repo.dbPool.DB(ctx, true).First(&subscription, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

func (repo *subscriptionRepository) GetByPropertyID(ctx context.Context, propertyId string, query string) ([]models.Subscription, error) {
	var subscriptionList []models.Subscription

	db := repo.dbPool.DB(ctx, true).Where("property_id = ?", propertyId)
	if query != "" {
		db = db.Where("role iLike ?", fmt.Sprintf("%%%s%%", query))
	}
	err := db.Find(&subscriptionList).Error
	if err != nil {
		return nil, err
	}

	return subscriptionList, nil
}

func (repo *subscriptionRepository) Save(ctx context.Context, subscription *models.Subscription) error {
	err := repo.dbPool.DB(ctx, false).Save(subscription).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repo.dbPool.DB(ctx, false).Create(subscription).Error
	}
	return err
}

func (repo *subscriptionRepository) Delete(ctx context.Context, id string) error {
	subscription, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return repo.dbPool.DB(ctx, false).Delete(subscription).Error
}
