package models

import (
	"fmt"
	"strconv"

	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/data"
)

const (
	encVersionKey      = "enc_v"
	encAlgKey          = "enc_alg"
	encChunkSizeKey    = "enc_chunk"
	encWrappedKeyKey   = "enc_key"
	encWrappedNonceKey = "enc_key_nonce"
	encNoncePrefixKey  = "enc_nonce_prefix"
)

// MediaMetadata Our model responsible for holding uploaded file data
type MediaMetadata struct {
	data.BaseModel

	OwnerID  string `gorm:"type:TEXT"`
	ParentID string `gorm:"type:TEXT"`

	Name string `gorm:"type:TEXT"`
	Ext  string `gorm:"type:TEXT"`

	Size       int64
	OriginTs   int64
	Public     bool
	Mimetype   string `gorm:"type:TEXT"`
	ServerName string `gorm:"type:TEXT"`

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
		ServerName:        mm.ServerName,
		IsPublic:          mm.Public,
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

	if mm.Properties != nil {
		tmm.Encryption = readEncryptionInfo(mm.Properties)
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
	mm.Public = tmm.IsPublic
	mm.ServerName = tmm.ServerName

	if tmm.ThumbnailSize != nil {
		if mm.Properties == nil {
			mm.Properties = make(data.JSONMap)
		}
		mm.Properties["h"] = fmt.Sprintf("%d", tmm.ThumbnailSize.Height)
		mm.Properties["w"] = fmt.Sprintf("%d", tmm.ThumbnailSize.Width)
		mm.Properties["m"] = tmm.ThumbnailSize.ResizeMethod
	}

	if tmm.Encryption != nil {
		if mm.Properties == nil {
			mm.Properties = make(data.JSONMap)
		}
		writeEncryptionInfo(mm.Properties, tmm.Encryption)
	}

}

func readEncryptionInfo(props data.JSONMap) *types.EncryptionInfo {
	if props == nil {
		return nil
	}

	versionStr := props.GetString(encVersionKey)
	if versionStr == "" {
		return nil
	}
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil
	}

	chunkStr := props.GetString(encChunkSizeKey)
	chunkSize := 0
	if chunkStr != "" {
		chunkSize, _ = strconv.Atoi(chunkStr)
	}

	return &types.EncryptionInfo{
		Version:         version,
		Algorithm:       props.GetString(encAlgKey),
		ChunkSizeBytes:  chunkSize,
		WrappedKey:      props.GetString(encWrappedKeyKey),
		WrappedKeyNonce: props.GetString(encWrappedNonceKey),
		NoncePrefix:     props.GetString(encNoncePrefixKey),
	}
}

func writeEncryptionInfo(props data.JSONMap, info *types.EncryptionInfo) {
	if props == nil || info == nil {
		return
	}
	props[encVersionKey] = fmt.Sprintf("%d", info.Version)
	props[encAlgKey] = info.Algorithm
	props[encChunkSizeKey] = fmt.Sprintf("%d", info.ChunkSizeBytes)
	props[encWrappedKeyKey] = info.WrappedKey
	props[encWrappedNonceKey] = info.WrappedKeyNonce
	props[encNoncePrefixKey] = info.NoncePrefix
}

// MediaAudit model responsible for holding events on a file
type MediaAudit struct {
	data.BaseModel
	FileID string `gorm:"type:TEXT"`
	Action string `gorm:"type:TEXT"`
	Source string `gorm:"type:TEXT"`
}
