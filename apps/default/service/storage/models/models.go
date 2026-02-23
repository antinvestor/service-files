package models

import (
	"fmt"
	"strconv"
	"time"

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

// MultipartUpload model for tracking multipart file uploads
type MultipartUpload struct {
	data.BaseModel
	OwnerID       string `gorm:"type:TEXT;not null"`
	MediaID       string `gorm:"type:TEXT;not null"`
	UploadName    string `gorm:"type:TEXT"`
	ContentType   string `gorm:"type:TEXT"`
	TotalSize     int64
	PartSize      int64
	PartCount     int
	UploadedParts int    `gorm:"default:0"`
	UploadState   string `gorm:"type:VARCHAR(20);default:'pending'"`
	ExpiresAt     *time.Time
	Metadata      data.JSONMap
}

// MultipartUploadPart model for individual upload parts
type MultipartUploadPart struct {
	data.BaseModel
	UploadID    string `gorm:"type:VARCHAR(50);not null;index:idx_multipart_upload_parts_upload_id"`
	PartNumber  int    `gorm:"not null;index:idx_multipart_upload_parts_upload_part,priority:1"`
	Etag        string `gorm:"type:TEXT"`
	Size        int64
	ContentHash string `gorm:"type:TEXT"`
	StoragePath string `gorm:"type:TEXT"`
	IsUploaded  bool   `gorm:"default:false"`
}

// FileVersion model for tracking file version history
type FileVersion struct {
	data.BaseModel
	MediaID            string `gorm:"type:TEXT;not null;index:idx_file_versions_media_id"`
	VersionNumber      int    `gorm:"not null;index:idx_file_versions_media_version,priority:2"`
	ContentHash        string `gorm:"type:TEXT;not null"`
	FileSize           int64
	UploadName         string `gorm:"type:TEXT"`
	ContentType        string `gorm:"type:TEXT"`
	StoragePath        string `gorm:"type:TEXT"`
	CreatedBy          string `gorm:"type:TEXT"`
	RestoreFromVersion int
}

// RetentionPolicy model for retention policies
type RetentionPolicy struct {
	data.BaseModel
	Name          string `gorm:"type:TEXT;not null"`
	Description   string `gorm:"type:TEXT"`
	RetentionDays int    `gorm:"not null"`
	IsDefault     bool   `gorm:"default:false;index:idx_retention_policies_is_default"`
	IsSystem      bool   `gorm:"default:false"`
	OwnerID       string `gorm:"type:TEXT;index:idx_retention_policies_owner_id"`
	Metadata      data.JSONMap
}

// FileRetention model for tracking retention assignments to files
type FileRetention struct {
	data.BaseModel
	MediaID   string     `gorm:"type:TEXT;not null;index:idx_file_retention_media_id"`
	PolicyID  string     `gorm:"type:VARCHAR(50);not null;index:idx_file_retention_policy_id"`
	AppliedAt time.Time  `gorm:"default:now()"`
	ExpiresAt *time.Time `gorm:"index:idx_file_retention_expires_at"`
	IsLocked  bool       `gorm:"default:false"`
	Metadata  data.JSONMap
}

// StorageStats model for tracking storage statistics
type StorageStats struct {
	data.BaseModel
	RecordDate   time.Time `gorm:"type:DATE;not null;index:idx_storage_stats_record_date"`
	TotalBytes   int64     `gorm:"default:0"`
	FileCount    int       `gorm:"default:0"`
	UserCount    int       `gorm:"default:0"`
	PublicBytes  int64     `gorm:"default:0"`
	PrivateBytes int64     `gorm:"default:0"`
	Metadata     data.JSONMap
}
