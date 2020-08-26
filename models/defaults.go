package models

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/rs/xid"
)

// Migration Our simple table holding all the migration data
type AntBaseModel struct {
	CreatedAt      time.Time
	ModifiedAt     time.Time
	Version        uint32 `gorm:"DEFAULT 0"`
	DeletedAt      *time.Time
}

func (model *AntBaseModel) IDGen( uniqueCode string, ) string{
	return fmt.Sprintf("%s_%s", uniqueCode, xid.New().String())
}
// BeforeCreate Ensures we update a migrations time stamps
func (model *AntBaseModel) BeforeCreate(scope *gorm.Scope) error {

	if err := scope.SetColumn("CreatedAt", time.Now()); err != nil{
		return err
	}
	if err := scope.SetColumn("ModifiedAt", time.Now()); err != nil{
		return err
	}
	return scope.SetColumn("Version", 1)
}

// BeforeUpdate Updates time stamp every time we update status of a migration
func (model *AntBaseModel) BeforeUpdate(scope *gorm.Scope) error {
	if err := scope.SetColumn("Version", model.Version+1); err != nil{
		return err
	}
	return scope.SetColumn("ModifiedAt", time.Now())
}

// Migration Our simple table holding all the migration data
type Migration struct {
	AntBaseModel

	MigrationID string `gorm:"type:varchar(50);primary_key"`
	Name        string `gorm:"type:varchar(50);unique_index"`
	Patch       string `gorm:"type:text"`
	AppliedAt   *time.Time
}

func (model *Migration) BeforeCreate(scope *gorm.Scope) error {

	if err := model.AntBaseModel.BeforeCreate(scope); err != nil{
		return err
	}
	return scope.SetColumn("MigrationID", model.IDGen("mg"))
}

