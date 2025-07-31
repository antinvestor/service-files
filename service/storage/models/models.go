package models

import (
	"strconv"

	"github.com/antinvestor/service-files/service/types"
	"github.com/pitabwire/frame"
)

// MediaMetadata Our model responsible for holding uploaded file data
type MediaMetadata struct {
	frame.BaseModel

	OwnerID  string `gorm:"type:TEXT"`
	ParentID string `gorm:"type:TEXT"`

	Name string `gorm:"type:TEXT"`
	Ext  string `gorm:"type:TEXT"`

	Size     int64
	OriginTs int64
	Public   bool
	Mimetype string `gorm:"type:TEXT"`

	Hash       string `gorm:"type:TEXT"`
	BucketName string `gorm:"type:TEXT"`
	Provider   string `gorm:"type:TEXT"`

	Properties frame.JSONMap
}

func (mm *MediaMetadata) ToApi() *types.MediaMetadata {

	tmm := types.MediaMetadata{
		MediaID:           types.MediaID(mm.GetID()),
		ContentType:       types.ContentType(mm.Mimetype),
		FileSizeBytes:     types.FileSizeBytes(mm.Size),
		CreationTimestamp: uint64(mm.OriginTs),
		UploadName:        types.Filename(mm.Name),
		Base64Hash:        types.Base64Hash(mm.Hash),
		OwnerID:           types.OwnerID(mm.OwnerID),
	}

	if mm.ParentID != "" {
		props := frame.DBPropertiesToMap(mm.Properties)
		tmm.ParentID = types.MediaID(mm.ParentID)

		h := 0
		h, _ = strconv.Atoi(props["h"])

		w := 0
		w, _ = strconv.Atoi(props["w"])

		tmm.ThumbnailSize = &types.ThumbnailSize{
			Width:        w,
			Height:       h,
			ResizeMethod: props["m"],
		}
	}

	return &tmm

}

func (mm *MediaMetadata) Fill(tmm *types.MediaMetadata) {
	mm.ID = string(tmm.MediaID)
	mm.ParentID = string(tmm.ParentID)
	mm.Size = int64(tmm.FileSizeBytes)
	mm.Name = string(tmm.UploadName)
	mm.Mimetype = string(tmm.ContentType)
	mm.OwnerID = string(tmm.OwnerID)
	mm.Hash = string(tmm.Base64Hash)
	mm.OriginTs = int64(tmm.CreationTimestamp)

	if tmm.ThumbnailSize != nil {

		props := frame.DBPropertiesToMap(mm.Properties)

		props["h"] = strconv.Itoa(tmm.ThumbnailSize.Height)
		props["w"] = strconv.Itoa(tmm.ThumbnailSize.Width)
		props["m"] = tmm.ThumbnailSize.ResizeMethod

		mm.Properties = frame.DBPropertiesFromMap(props)
	}

}

// MediaAudit model responsible for holding events on a file
type MediaAudit struct {
	frame.BaseModel
	FileID   string `gorm:"type:TEXT"`
	AccessID string `gorm:"type:TEXT"`
	Action   string `gorm:"type:TEXT"`
	Source   string `gorm:"type:TEXT"`
}
