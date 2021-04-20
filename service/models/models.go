package models

import (
	"fmt"
	"github.com/antinvestor/files/openapi"
	"github.com/pitabwire/frame"
)

// File Our model responsible for holding uploaded file data
type File struct {
	frame.BaseModel

	GroupID        string `gorm:"type:varchar(50)"`
	SubscriptionID string `gorm:"type:varchar(50)"`

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

func (file *File) ToApi(fileAccessServer string) openapi.File {

	fileUrl := fmt.Sprintf("%s/%s", fileAccessServer, file.ID)

	return openapi.File{
		Id:             file.ID,
		Name:           file.Name,
		Public:         file.Public,
		GroupId:        file.GroupID,
		SubscriptionId: file.SubscriptionID,
		Url:            fileUrl,
	}
}

// AuditFile model responsible for holding events on a file
type AuditFile struct {
	frame.BaseModel
	FileID         string `gorm:"type:varchar(50)"`
	SubscriptionID string `gorm:"type:varchar(50)"`
	Action         string `gorm:"type:varchar(250)"`
	Source         string `gorm:"type:varchar(50)"`

}
