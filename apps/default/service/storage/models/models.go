package models

import (
	"fmt"
	"strconv"

	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/data"
)

// MediaMetadata Our model responsible for holding uploaded file data
type MediaMetadata struct {
	data.BaseModel

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

	Properties data.JSONMap
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
		tmm.ParentID = types.MediaID(mm.ParentID)

		h := 0
		if hStr := mm.Properties.GetString("h"); hStr != "" {
			if hParsed, err := strconv.Atoi(hStr); err == nil {
				h = hParsed
			}
		}

		w := 0
		if wStr := mm.Properties.GetString("w"); wStr != "" {
			if wParsed, err := strconv.Atoi(wStr); err == nil {
				w = wParsed
			}
		}

		tmm.ThumbnailSize = &types.ThumbnailSize{
			Width:        w,
			Height:       h,
			ResizeMethod: mm.Properties.GetString("m"),
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
		if mm.Properties == nil {
			mm.Properties = make(data.JSONMap)
		}
		mm.Properties["h"] = fmt.Sprintf("%d", tmm.ThumbnailSize.Height)
		mm.Properties["w"] = fmt.Sprintf("%d", tmm.ThumbnailSize.Width)
		mm.Properties["m"] = tmm.ThumbnailSize.ResizeMethod
	}

}

// MediaAudit model responsible for holding events on a file
type MediaAudit struct {
	data.BaseModel
	FileID   string `gorm:"type:TEXT"`
	AccessID string `gorm:"type:TEXT"`
	Action   string `gorm:"type:TEXT"`
	Source   string `gorm:"type:TEXT"`
}
