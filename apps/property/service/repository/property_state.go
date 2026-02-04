package repository

import (
	"context"
	"errors"

	"github.com/antinvestor/service-files/apps/property/service/models"
	"github.com/pitabwire/frame/datastore/pool"
	"gorm.io/gorm"
)

type PropertyStateRepository interface {
	GetByID(ctx context.Context, id string) (*models.PropertyState, error)
	GetByPropertyID(ctx context.Context, id string) (*models.PropertyState, error)
	GetAllByPropertyID(ctx context.Context, id string) ([]models.PropertyState, error)
	Save(ctx context.Context, propertyState *models.PropertyState) error
}

type propertyStateRepository struct {
	dbPool pool.Pool
}

func NewPropertyStateRepository(dbPool pool.Pool) PropertyStateRepository {
	return &propertyStateRepository{dbPool: dbPool}
}

func (repo *propertyStateRepository) GetByPropertyID(ctx context.Context, id string) (*models.PropertyState, error) {
	var propertyState models.PropertyState
	err := repo.dbPool.DB(ctx, true).Last(&propertyState, "property_id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &propertyState, nil
}

func (repo *propertyStateRepository) GetAllByPropertyID(ctx context.Context, id string) ([]models.PropertyState, error) {
	var propertyStates []models.PropertyState
	err := repo.dbPool.DB(ctx, true).Find(&propertyStates, "property_id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return propertyStates, nil
}

func (repo *propertyStateRepository) GetByID(ctx context.Context, id string) (*models.PropertyState, error) {
	propertyState := models.PropertyState{}
	err := repo.dbPool.DB(ctx, true).First(&propertyState, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &propertyState, nil
}

func (repo *propertyStateRepository) Save(ctx context.Context, propertyState *models.PropertyState) error {
	err := repo.dbPool.DB(ctx, false).Save(propertyState).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repo.dbPool.DB(ctx, false).Create(propertyState).Error
	}
	return err
}
