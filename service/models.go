package service

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/rs/xid"
)

// AntMigration Our simple table holding all the migration data
type AntMigration struct {
	ID         uint   `gorm:"primary_key"`
	Name       string `gorm:"type:varchar(50);unique_index"`
	Patch      string `gorm:"type:text"`
	AppliedAt  *time.Time
	CreatedAt  time.Time
	ModifiedAt time.Time
	Version    uint32 `gorm:"DEFAULT 0"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *AntMigration) BeforeCreate(scope *gorm.Scope) error {
	scope.SetColumn("CreatedAt", time.Now())
	return scope.SetColumn("ModifiedAt", time.Now())
}

// BeforeUpdate Updates time stamp every time we update status of a migration
func (model *AntMigration) BeforeUpdate(scope *gorm.Scope) error {
	scope.SetColumn("Version", model.Version+1)
	return scope.SetColumn("ModifiedAt", time.Now())
}

// File Our model responsible for holding uploaded file data
type File struct {
	FileID string `gorm:"primary_key"`

	GroupID        string `gorm:"type:varchar(50)"`
	SubscriptionID string `gorm:"type:varchar(50)"`

	Name string `gorm:"type:varchar(250)"`
	Ext  string `gorm:"type:varchar(10)"`

	URL         string `gorm:"type:varchar(255)"`
	Size        uint64
	Description string `gorm:"type:text"`
	Public      bool
	Mimetype    string `gorm:"type:varchar(250)"`

	Hash       string `gorm:"type:varchar(255)"`
	BucketName string `gorm:"type:varchar(255)"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
	Version   uint32     `gorm:"DEFAULT 0"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *File) BeforeCreate(scope *gorm.Scope) error {
	scope.SetColumn("FileID", xid.New().String())
	scope.SetColumn("CreatedAt", time.Now())
	return scope.SetColumn("ModifiedAt", time.Now())
}

// BeforeUpdate Updates time stamp every time we update status of a migration
func (model *File) BeforeUpdate(scope *gorm.Scope) error {
	scope.SetColumn("Version", model.Version+1)
	return scope.SetColumn("ModifiedAt", time.Now())
}

// AuditFile model responsible for holding events on a file
type AuditFile struct {
	AuditFileID    string `gorm:"type:primary_key"`
	FileID         string `gorm:"type:varchar(50)"`
	SubscriptionID string `gorm:"type:varchar(50)"`
	Action         string `gorm:"type:varchar(250)"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`

	Version uint32 `gorm:"DEFAULT 0"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *AuditFile) BeforeCreate(scope *gorm.Scope) error {
	scope.SetColumn("AuditFileID", xid.New().String())
	scope.SetColumn("CreatedAt", time.Now())
	return scope.SetColumn("ModifiedAt", time.Now())
}

// BeforeUpdate Updates time stamp every time we update status of a migration
func (model *AuditFile) BeforeUpdate(scope *gorm.Scope) error {
	scope.SetColumn("Version", model.Version+1)
	return scope.SetColumn("ModifiedAt", time.Now())
}
