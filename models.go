package main

import (
	"time"
)

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
}

// AuditFile model responsible for holding events on a file
type AuditFile struct {
	AuditFileID    string `gorm:"type:varchar(50)"`
	FileID         string `gorm:"type:varchar(50)"`
	SubscriptionID string `gorm:"type:varchar(50)"`
	Action         string `gorm:"type:varchar(250)"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
}
