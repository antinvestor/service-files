package business

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/queue/thumbnailer"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/util"
	"golang.org/x/sync/singleflight"
)

// mediaService implements the MediaService interface
type mediaService struct {
	db       storage.Database
	provider storage.Provider

	thumbnailGroup singleflight.Group
}

// NewMediaService creates a new instance of the media service
func NewMediaService(db storage.Database, provider storage.Provider) MediaService {
	return &mediaService{
		db:       db,
		provider: provider,
	}
}

// GenerateMediaID generates a new unique media ID
func (s *mediaService) GenerateMediaID(ctx context.Context) types.MediaID {
	return types.MediaID(utils.GenerateRandomString(32))
}

// UploadFile implements the business logic for uploading a file
func (s *mediaService) UploadFile(ctx context.Context, req *UploadRequest) (*UploadResult, error) {
	// Validate the upload request
	if err := s.validateUploadRequest(req); err != nil {
		return nil, err
	}

	// Process the upload
	result, err := s.processUpload(ctx, req)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// DownloadFile implements the business logic for downloading a file
func (s *mediaService) DownloadFile(ctx context.Context, req *DownloadRequest) (*DownloadResult, error) {
	// Validate the download request
	if err := s.validateDownloadRequest(req); err != nil {
		return nil, err
	}

	mediaMetadata, err := s.db.GetMediaMetadata(ctx, req.MediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media metadata: %w", err)
	}
	if mediaMetadata == nil {
		return nil, fmt.Errorf("media not found")
	}

	if req.IsThumbnailRequest {
		thumbnailMetadata, err := s.resolveThumbnail(ctx, req, mediaMetadata)
		if err != nil {
			return nil, err
		}
		mediaMetadata = thumbnailMetadata.MediaMetadata
	}

	// Get the file data
	fileData, contentLength, contentType, err := s.getFileData(ctx, mediaMetadata, req.Config)
	if err != nil {
		return nil, err
	}

	// Determine filename
	filename := s.getDownloadFilename(mediaMetadata, req.DownloadFilename)

	return &DownloadResult{
		MediaMetadata: mediaMetadata,
		FileData:      fileData,
		ContentType:   contentType,
		ContentLength: contentLength,
		Filename:      filename,
		IsCached:      string(mediaMetadata.ServerName) == req.Config.ServerName,
	}, nil
}

// SearchMedia implements the business logic for searching media files
func (s *mediaService) SearchMedia(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	// Validate search request
	if err := s.validateSearchRequest(req); err != nil {
		return nil, err
	}

	filtersAnd := map[string]interface{}{
		"owner_id = ?": req.OwnerID,
	}
	if req.ParentID != "" {
		filtersAnd["parent_id = ?"] = req.ParentID
	}
	if req.StartDate != nil {
		filtersAnd["created_at >= ?"] = *req.StartDate
	}
	if req.EndDate != nil {
		filtersAnd["created_at <= ?"] = *req.EndDate
	}
	filtersOr := map[string]any{}
	if strings.TrimSpace(req.Query) != "" {
		query := strings.TrimSpace(req.Query)
		likeQuery := "%" + query + "%"
		filtersOr["name ILIKE ?"] = likeQuery
		filtersOr["mimetype ILIKE ?"] = likeQuery
		filtersOr["search_properties @@ plainto_tsquery(?)"] = query
	}

	searchQuery := data.NewSearchQuery(
		data.WithSearchFiltersAndByValue(filtersAnd),
		data.WithSearchFiltersOrByValue(filtersOr),
		data.WithSearchLimit(int(req.Limit)+1),
		data.WithSearchOffset(int(req.Page*req.Limit)),
	)
	searchQuery.OrderBy = "created_at DESC"

	results, err := s.db.Search(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to search media: %w", err)
	}

	// Read results from the pipe
	mediaResults := make([]*types.MediaMetadata, 0)
	for {
		result, ok := results.ReadResult(ctx)
		if !ok {
			break
		}
		if result.IsError() {
			return nil, result.Error()
		}
		mediaResults = append(mediaResults, result.Item())
	}

	// Determine if there are more results
	hasMore := len(mediaResults) > int(req.Limit)
	if hasMore {
		mediaResults = mediaResults[:int(req.Limit)]
	}

	return &SearchResult{
		Results: mediaResults,
		Count:   len(mediaResults),
		Page:    int(req.Page),
		HasMore: hasMore,
	}, nil
}

// validateUploadRequest validates the upload request
func (s *mediaService) validateUploadRequest(req *UploadRequest) error {
	// Check file size
	if req.Config.MaxFileSizeBytes > 0 && req.FileSizeBytes > types.FileSizeBytes(req.Config.MaxFileSizeBytes) {
		return fmt.Errorf("invalid parameter: HTTP Content-Length is greater than the maximum allowed upload size (%v)", req.Config.MaxFileSizeBytes)
	}

	// Check filename
	if strings.HasPrefix(string(req.UploadName), "~") {
		return fmt.Errorf("invalid parameter: File name must not begin with '~'")
	}
	if path.Base(string(req.UploadName)) != string(req.UploadName) {
		return fmt.Errorf("invalid parameter: File name must not contain path separators")
	}

	if req.MediaID != "" && !isValidMediaID(string(req.MediaID)) {
		return fmt.Errorf("invalid parameter: mediaId must be a non-empty string and contain only characters A-Za-z0-9_=-")
	}

	// Validate user ID

	return nil
}

// processUpload handles the actual file upload process
func (s *mediaService) processUpload(ctx context.Context, req *UploadRequest) (*UploadResult, error) {
	logger := util.Log(ctx).With(
		"UploadName", req.UploadName,
		"FileSizeBytes", req.FileSizeBytes,
		"ContentType", req.ContentType,
	)
	logger.Debug("Uploading file")

	// Limit file size if configured
	reader := req.FileData
	if req.Config.MaxFileSizeBytes > 0 {
		if req.Config.MaxFileSizeBytes+1 <= 0 {
			logger.With(
				"MaxFileSizeBytes", req.Config.MaxFileSizeBytes,
				"Default File SizeBytes", config.DefaultMaxFileSizeBytes,
			).Warn("Configured MaxFileSizeBytes overflows int64")
			req.Config.MaxFileSizeBytes = config.DefaultMaxFileSizeBytes
		}
		reader = io.LimitReader(reader, int64(req.Config.MaxFileSizeBytes)+1)
	}

	// Write file to temporary location
	hash, bytesWritten, tmpDir, err := utils.WriteTempFile(ctx, reader, req.Config.AbsBasePath)
	if err != nil {
		logger.WithError(err).With(
			"MaxFileSizeBytes", req.Config.MaxFileSizeBytes,
		).Warn("Error while transferring file")
		return nil, fmt.Errorf("invalid parameter: Failed to upload")
	}

	// Check if file size exceeds limit
	if req.Config.MaxFileSizeBytes > 0 && bytesWritten > types.FileSizeBytes(req.Config.MaxFileSizeBytes) {
		utils.RemoveDir(tmpDir, logger)
		return nil, fmt.Errorf("invalid parameter: HTTP Content-Length is greater than the maximum allowed upload size (%v)", req.Config.MaxFileSizeBytes)
	}

	// Check if file already exists by hash
	existingMetadata, err := s.db.GetMediaMetadataByHash(ctx, req.OwnerID, hash)
	if err != nil {
		utils.RemoveDir(tmpDir, logger)
		logger.WithError(err).Error("Error querying the database by hash.")
		return nil, fmt.Errorf("internal server error")
	}

	if req.MediaID != "" {
		existingByID, err := s.db.GetMediaMetadata(ctx, req.MediaID)
		if err != nil {
			utils.RemoveDir(tmpDir, logger)
			return nil, fmt.Errorf("internal server error")
		}
		if existingByID != nil {
			utils.RemoveDir(tmpDir, logger)
			return nil, fmt.Errorf("media already exists")
		}
	}

	var mediaMetadata *types.MediaMetadata
	reusedExistingMetadata := false
	if existingMetadata != nil {
		// File already exists, use existing metadata
		defer utils.RemoveDir(tmpDir, logger)
		mediaMetadata = existingMetadata
		reusedExistingMetadata = true
	} else {
		// New file, create metadata
		mediaID := req.MediaID
		if mediaID == "" {
			mediaID = s.generateMediaID(ctx)
		}
		mediaMetadata = &types.MediaMetadata{
			MediaID:           mediaID,
			UploadName:        req.UploadName,
			ContentType:       req.ContentType,
			FileSizeBytes:     bytesWritten,
			Base64Hash:        hash,
			OwnerID:           req.OwnerID,
			ServerName:        req.Config.ServerName,
			IsPublic:          req.IsPublic,
			CreationTimestamp: uint64(time.Now().UnixMilli()),
		}
	}

	logger.WithField("media_id", mediaMetadata.MediaID).With(
		"Base64Hash", mediaMetadata.Base64Hash,
		"UploadName", mediaMetadata.UploadName,
		"FileSizeBytes", mediaMetadata.FileSizeBytes,
		"ContentType", mediaMetadata.ContentType,
	).Info("File uploaded")

	if reusedExistingMetadata {
		return &UploadResult{
			MediaID:    mediaMetadata.MediaID,
			ServerName: string(mediaMetadata.ServerName),
			ContentURI: fmt.Sprintf("mxc://%s/%s", mediaMetadata.ServerName, mediaMetadata.MediaID),
		}, nil
	}

	// Store file and metadata
	err = s.storeFileAndMetadata(ctx, tmpDir, mediaMetadata, req.Config)
	if err != nil {
		logger.WithError(err).Error("Failed to upload file.")
		return nil, fmt.Errorf("invalid parameter: %s", err.Error())
	}

	return &UploadResult{
		MediaID:    mediaMetadata.MediaID,
		ServerName: string(mediaMetadata.ServerName),
		ContentURI: fmt.Sprintf("mxc://%s/%s", mediaMetadata.ServerName, mediaMetadata.MediaID),
	}, nil
}

// validateDownloadRequest validates the download request
func (s *mediaService) validateDownloadRequest(req *DownloadRequest) error {
	// Validate media ID format
	if !isValidMediaID(string(req.MediaID)) {
		return fmt.Errorf("invalid parameter: mediaId must be a non-empty string and contain only characters A-Za-z0-9_=-")
	}

	// Validate thumbnail parameters if it's a thumbnail request
	if req.IsThumbnailRequest {
		if req.ThumbnailSize == nil {
			return fmt.Errorf("invalid parameter: thumbnail size is required")
		}
		if req.ThumbnailSize.Width <= 0 || req.ThumbnailSize.Height <= 0 {
			return fmt.Errorf("invalid parameter: width and height must be > 0")
		}
		maxDim := req.Config.MaxThumbnailDimension
		if maxDim == 0 {
			maxDim = 2048
		}
		if req.ThumbnailSize.Width > maxDim || req.ThumbnailSize.Height > maxDim {
			return fmt.Errorf("invalid parameter: width and height must be <= %d", maxDim)
		}
		if req.ThumbnailSize.ResizeMethod != types.Crop && req.ThumbnailSize.ResizeMethod != types.Scale {
			return fmt.Errorf("invalid parameter: unsupported resize method")
		}
	}

	return nil
}

// validateSearchRequest validates the search request
func (s *mediaService) validateSearchRequest(req *SearchRequest) error {
	if req.Page < 0 {
		return fmt.Errorf("invalid parameter: page must be >= 0")
	}
	if req.Limit <= 0 || req.Limit > 1000 {
		return fmt.Errorf("invalid parameter: limit must be > 0 and <= 1000")
	}
	return nil
}

// getFileData retrieves the file data for the given media metadata
func (s *mediaService) getFileData(ctx context.Context, mediaMetadata *types.MediaMetadata, cfg *config.FilesConfig) (io.ReadCloser, int64, string, error) {
	// Get the file path from media metadata hash
	filePath, err := utils.GetPathFromBase64Hash(mediaMetadata.Base64Hash, cfg.AbsBasePath)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to get file path from metadata: %w", err)
	}

	// Determine bucket based on media properties
	bucket := s.provider.GetBucket(mediaMetadata.IsPublic)

	// Download file from storage
	reader, cleanup, err := s.provider.DownloadFile(ctx, bucket, types.Path(filePath))
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to get file from storage: %w", err)
	}

	// Create a ReadCloser that calls cleanup when closed
	readCloser := &readCloserWithCleanup{
		Reader:  reader,
		cleanup: cleanup,
	}

	if mediaMetadata.Encryption != nil {
		decryptingReader, err := storage.NewDecryptingReader(reader, []byte(cfg.EnvStorageEncryptionPhrase), mediaMetadata.Encryption)
		if err != nil {
			cleanup()
			return nil, 0, "", fmt.Errorf("failed to initialise decrypting reader: %w", err)
		}
		readCloser = &readCloserWithCleanup{
			Reader:  decryptingReader,
			cleanup: cleanup,
		}
	}

	// Determine content type
	contentType := string(mediaMetadata.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return readCloser, int64(mediaMetadata.FileSizeBytes), contentType, nil
}

// readCloserWithCleanup wraps an io.Reader with a cleanup function
type readCloserWithCleanup struct {
	io.Reader
	cleanup func()
}

func (rc *readCloserWithCleanup) Close() error {
	rc.cleanup()
	return nil
}

// getDownloadFilename determines the appropriate filename for download
func (s *mediaService) getDownloadFilename(mediaMetadata *types.MediaMetadata, customFilename string) string {
	if customFilename != "" {
		return customFilename
	}
	if mediaMetadata.UploadName != "" {
		return string(mediaMetadata.UploadName)
	}
	return string(mediaMetadata.MediaID)
}

// generateMediaID generates a new media ID
func (s *mediaService) generateMediaID(ctx context.Context) types.MediaID {
	model := data.BaseModel{}
	model.GenID(ctx)
	return types.MediaID(model.GetID())
}

func (s *mediaService) encryptToPath(
	ctx context.Context,
	sourcePath types.Path,
	encryptedPath types.Path,
	mediaMetadata *types.MediaMetadata,
	cfg *config.FilesConfig,
) error {
	srcFile, err := os.Open(string(sourcePath))
	if err != nil {
		return err
	}
	defer util.CloseAndLogOnError(ctx, srcFile)

	dstFile, err := os.Create(string(encryptedPath))
	if err != nil {
		return err
	}
	defer util.CloseAndLogOnError(ctx, dstFile)

	info, err := storage.EncryptStream(ctx, srcFile, dstFile, []byte(cfg.EnvStorageEncryptionPhrase))
	if err != nil {
		return err
	}
	mediaMetadata.Encryption = info
	return nil
}

func (s *mediaService) resolveThumbnail(ctx context.Context, req *DownloadRequest, mediaMetadata *types.MediaMetadata) (*types.ThumbnailMetadata, error) {
	cfg := req.Config
	thumbnailSize := req.ThumbnailSize
	if thumbnailSize == nil {
		return nil, fmt.Errorf("thumbnail size is required")
	}

	thumbnails, err := s.db.GetThumbnails(ctx, req.MediaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thumbnails: %w", err)
	}

	chosenThumbnail, sizeToGenerate := thumbnailer.SelectThumbnail(*thumbnailSize, thumbnails, cfg.ThumbnailSizes)
	if chosenThumbnail != nil {
		return chosenThumbnail, nil
	}

	if sizeToGenerate == nil {
		return nil, fmt.Errorf("thumbnail not found")
	}

	if !cfg.DynamicThumbnails {
		if !thumbnailSizeEqual(*sizeToGenerate, *thumbnailSize) {
			return nil, fmt.Errorf("thumbnail not found")
		}
	}

	_, err, _ = s.thumbnailGroup.Do(thumbnailKey(req.MediaID, *sizeToGenerate), func() (any, error) {
		return nil, thumbnailer.GenerateThumbnail(ctx, *sizeToGenerate, mediaMetadata, cfg.AbsBasePath, s.db, s.provider, util.Log(ctx), cfg.EnvStorageEncryptionPhrase)
	})
	if err != nil {
		return nil, err
	}

	thumbnailMetadata, err := s.db.GetThumbnail(ctx, req.MediaID, sizeToGenerate.Width, sizeToGenerate.Height, sizeToGenerate.ResizeMethod)
	if err != nil {
		return nil, err
	}
	if thumbnailMetadata == nil {
		return nil, fmt.Errorf("thumbnail generation failed")
	}
	return thumbnailMetadata, nil
}

func thumbnailKey(mediaID types.MediaID, size types.ThumbnailSize) string {
	return fmt.Sprintf("%s:%dx%d:%s", mediaID, size.Width, size.Height, size.ResizeMethod)
}

func thumbnailSizeEqual(a types.ThumbnailSize, b types.ThumbnailSize) bool {
	return a.Width == b.Width && a.Height == b.Height && a.ResizeMethod == b.ResizeMethod
}

// storeFileAndMetadata stores the file and metadata in the database and storage
func (s *mediaService) storeFileAndMetadata(ctx context.Context, tmpDir types.Path, mediaMetadata *types.MediaMetadata, cfg *config.FilesConfig) error {
	logger := util.Log(ctx)
	defer utils.RemoveDir(tmpDir, logger)

	sourcePath := types.Path(filepath.Join(string(tmpDir), "content"))
	if !mediaMetadata.IsPublic {
		encryptedPath := types.Path(filepath.Join(string(tmpDir), "content.encrypted"))
		if err := s.encryptToPath(ctx, sourcePath, encryptedPath, mediaMetadata, cfg); err != nil {
			return err
		}
		sourcePath = encryptedPath
	}

	finalPath, duplicate, err := storage.UploadFileWithHashCheck(ctx, s.provider, sourcePath, mediaMetadata, cfg.AbsBasePath, logger)
	if err != nil {
		return err
	}

	if duplicate {
		logger.WithField("dst", finalPath).Info("File was stored previously - discarding duplicate")
	}

	if err = s.db.StoreMediaMetadata(ctx, mediaMetadata); err != nil {
		logger.WithError(err).Warn("Failed to store metadata")
		// Clean up file if it's not a duplicate
		if !duplicate {
			utils.RemoveDir(types.Path(path.Dir(string(finalPath))), logger)
		}
		return err
	}

	return nil
}

// isValidMediaID checks if the media ID is valid
func isValidMediaID(mediaID string) bool {
	if mediaID == "" {
		return false
	}
	// Check if all characters are valid
	for _, r := range mediaID {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '=' && r != '-' {
			return false
		}
	}
	return true
}
