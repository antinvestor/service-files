package service

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/rs/xid"
	"github.com/antinvestor/files/openapi"
	"fmt"
)

// AntMigration Our simple table holding all the migration data
type AntMigration struct {
	AntMigrationID string `gorm:"type:varchar(50);primary_key"`
	Name           string `gorm:"type:varchar(50);unique_index"`
	Patch          string `gorm:"type:text"`
	AppliedAt      *time.Time
	CreatedAt      time.Time
	ModifiedAt     time.Time
	Version        uint32 `gorm:"DEFAULT 0"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *AntMigration) BeforeCreate(scope *gorm.Scope) error {
	scope.SetColumn("AntMigrationID", xid.New().String())
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
	FileID string `gorm:"type:varchar(50);primary_key"`

	GroupID        string `gorm:"type:varchar(50)"`
	SubscriptionID string `gorm:"type:varchar(50)"`

	Name string `gorm:"type:varchar(250)"`
	Ext  string `gorm:"type:varchar(10)"`

	Size     int64
	Public   bool
	Mimetype string `gorm:"type:varchar(250)"`

	Hash       string `gorm:"type:varchar(255)"`
	BucketName string `gorm:"type:varchar(255)"`
	Provider      string `gorm:"type:varchar(50)"`
	UploadResult      string `gorm:"type:varchar(255)"`

	CreatedAt  time.Time
	ModifiedAt time.Time
	DeletedAt  *time.Time `sql:"index"`
	Version    int        `gorm:"DEFAULT 0"`
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

func (model *File) ToApi(env *Env) openapi.File  {

	fileUrl := fmt.Sprintf("%s/%s", env.FileAccessServer, model.FileID )

	return openapi.File{
		Id: model.FileID,
		Name: model.Name,
		Public: model.Public,
		GroupId: model.GroupID,
		SubscriptionId:model.SubscriptionID,
		Url: fileUrl,
	}
}

// AuditFile model responsible for holding events on a file
type AuditFile struct {
	AuditFileID    string `gorm:"type:varchar(50);primary_key"`
	FileID         string `gorm:"type:varchar(50)"`
	SubscriptionID string `gorm:"type:varchar(50)"`
	Action         string `gorm:"type:varchar(250)"`
	Source         string `gorm:"type:varchar(50)"`
	CreatedAt      time.Time
	ModifiedAt     time.Time
	DeletedAt      *time.Time `sql:"index"`

	Version int `gorm:"DEFAULT 0"`
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
