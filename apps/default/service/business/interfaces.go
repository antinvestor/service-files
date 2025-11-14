package business

import (
	"context"
	"io"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/types"
)

// MediaService defines the business logic interface for media operations
type MediaService interface {
	// UploadFile handles the business logic for uploading a file
	UploadFile(ctx context.Context, req *UploadRequest) (*UploadResult, error)
	
	// DownloadFile handles the business logic for downloading a file
	DownloadFile(ctx context.Context, req *DownloadRequest) (*DownloadResult, error)
	
	// SearchMedia handles the business logic for searching media files
	SearchMedia(ctx context.Context, req *SearchRequest) (*SearchResult, error)
}

// UploadRequest contains all the data needed for an upload operation
type UploadRequest struct {
	OwnerID       types.OwnerID
	UploadName    types.Filename
	ContentType   types.ContentType
	FileSizeBytes types.FileSizeBytes
	FileData      io.Reader
	Config        *config.FilesConfig
}

// UploadResult contains the result of an upload operation
type UploadResult struct {
	MediaID    types.MediaID
	ServerName string
	ContentURI string
}

// DownloadRequest contains all the data needed for a download operation
type DownloadRequest struct {
	MediaID            types.MediaID
	IsThumbnailRequest bool
	ThumbnailSize      *types.ThumbnailSize
	DownloadFilename   string
	Config             *config.FilesConfig
}

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	MediaMetadata *types.MediaMetadata
	FileData      io.ReadCloser
	ContentType   string
	ContentLength int64
	Filename      string
	IsCached      bool
}

// SearchRequest contains all the data needed for a search operation
type SearchRequest struct {
	OwnerID types.OwnerID
	Query   string
	Page    int32
	Limit   int32
}

// SearchResult contains the result of a search operation
type SearchResult struct {
	Results []*types.MediaMetadata
	Count   int
	Page    int
	HasMore bool
}
