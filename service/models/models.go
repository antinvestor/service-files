package models

import (
	"github.com/pitabwire/frame"
)

// File Our model responsible for holding uploaded file data
type File struct {
	frame.BaseModel

	GroupID  string `gorm:"type:varchar(50)"`
	AccessID string `gorm:"type:varchar(50)"`

	Name string `gorm:"type:varchar(250)"`
	Ext  string `gorm:"type:varchar(10)"`

	Size     int64
	Public   bool
	Mimetype string `gorm:"type:varchar(250)"`

	Hash         string `gorm:"type:varchar(255)"`
	BucketName   string `gorm:"type:varchar(255)"`
	Provider     string `gorm:"type:varchar(50)"`
	UploadResult string `gorm:"type:varchar(255)"`
}

// FileAudit model responsible for holding events on a file
type FileAudit struct {
	frame.BaseModel
	FileID   string `gorm:"type:varchar(50)"`
	AccessID string `gorm:"type:varchar(50)"`
	Action   string `gorm:"type:varchar(250)"`
	Source   string `gorm:"type:varchar(50)"`
}
