// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antinvestor/gomatrixserverlib/spec"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/queue/thumbnailer"
	"github.com/antinvestor/service-files/service/storage"
	"github.com/antinvestor/service-files/service/types"
	"github.com/antinvestor/service-files/service/utils"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/pitabwire/util"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const mediaIDCharacters = "A-Za-z0-9_=-"

// Note: unfortunately regex.MustCompile() cannot be assigned to a const
var mediaIDRegex = regexp.MustCompile("^[" + mediaIDCharacters + "]+$")

// Regular expressions to help us cope with Content-Disposition parsing
//var rfc2183 = regexp.MustCompile(`filename=utf-8"(.*)"`)
//var rfc6266 = regexp.MustCompile(`filename\*=utf-8''(.*)`)

// downloadRequest metadata included in or derivable from a download or thumbnail request
// https://matrix.org/docs/spec/client_server/r0.2.0.html#get-matrix-media-r0-download-servername-mediaid
// http://matrix.org/docs/spec/client_server/r0.2.0.html#get-matrix-media-r0-thumbnail-servername-mediaid
type downloadRequest struct {
	MediaMetadata      *types.MediaMetadata
	IsThumbnailRequest bool
	ThumbnailSize      types.ThumbnailSize
	Logger             *log.Entry
	DownloadFilename   string
	multipartResponse  bool // whether we need to return a multipart/mixed response (for requests coming in over federation)
}

// Taken from: https://github.com/matrix-org/synapse/blob/c3627d0f99ed5a23479305dc2bd0e71ca25ce2b1/synapse/media/_base.py#L53C1-L84
// A list of all content types that are "safe" to be rendered inline in a browser.
var allowInlineTypes = map[types.ContentType]struct{}{
	"text/css":            {},
	"text/plain":          {},
	"text/csv":            {},
	"application/json":    {},
	"application/ld+json": {},
	// We allow some media files deemed as safe, which comes from the matrix-react-sdk.
	// https://github.com/matrix-org/matrix-react-sdk/blob/a70fcfd0bcf7f8c85986da18001ea11597989a7c/src/utils/blobs.ts#L51
	// SVGs are *intentionally* omitted.
	"image/jpeg":      {},
	"image/gif":       {},
	"image/png":       {},
	"image/apng":      {},
	"image/webp":      {},
	"image/avif":      {},
	"video/mp4":       {},
	"video/webm":      {},
	"video/ogg":       {},
	"video/quicktime": {},
	"audio/mp4":       {},
	"audio/webm":      {},
	"audio/aac":       {},
	"audio/mpeg":      {},
	"audio/ogg":       {},
	"audio/wave":      {},
	"audio/wav":       {},
	"audio/x-wav":     {},
	"audio/x-pn-wav":  {},
	"audio/flac":      {},
	"audio/x-flac":    {},
}

// Download implements GET /download and GET /thumbnail
// Files from this server (i.e. origin == cfg.ServerName) are served directly
// Files from remote servers (i.e. origin != cfg.ServerName) are cached locally.
// If they are present in the cache, they are served directly.
// If they are not present in the cache, they are obtained from the remote server and
// simultaneously served back to the client and written into the cache.
func Download(
	w http.ResponseWriter,
	req *http.Request,
	mediaID types.MediaID,
	cfg *config.FilesConfig,
	db storage.Database,
	activeThumbnailGeneration *types.ActiveThumbnailGeneration,
	isThumbnailRequest bool,
	customFilename string,
) {

	dReq := &downloadRequest{
		MediaMetadata: &types.MediaMetadata{
			MediaID: mediaID,
		},
		IsThumbnailRequest: isThumbnailRequest,
		Logger: util.GetLogger(req.Context()).WithFields(log.Fields{
			"MediaID": mediaID,
		}),
		DownloadFilename:  customFilename,
		multipartResponse: false,
	}

	if dReq.IsThumbnailRequest {
		width, err := strconv.Atoi(req.FormValue("width"))
		if err != nil {
			width = -1
		}
		height, err := strconv.Atoi(req.FormValue("height"))
		if err != nil {
			height = -1
		}
		dReq.ThumbnailSize = types.ThumbnailSize{
			Width:        width,
			Height:       height,
			ResizeMethod: strings.ToLower(req.FormValue("method")),
		}
		dReq.Logger.WithFields(log.Fields{
			"RequestedWidth":        dReq.ThumbnailSize.Width,
			"RequestedHeight":       dReq.ThumbnailSize.Height,
			"RequestedResizeMethod": dReq.ThumbnailSize.ResizeMethod,
		})
	}

	// request validation
	if resErr := dReq.Validate(); resErr != nil {
		dReq.jsonErrorResponse(w, *resErr)
		return
	}

	metadata, err := dReq.doDownload(
		req.Context(), w, cfg, db, activeThumbnailGeneration,
	)
	if err != nil {
		// If we bubbled up a os.PathError, e.g. no such file or directory, don't send
		// it to the client, be more generic.
		var perr *fs.PathError
		if errors.As(err, &perr) {
			dReq.Logger.WithError(err).Error("failed to open file")
			dReq.jsonErrorResponse(w, util.JSONResponse{
				Code: http.StatusNotFound,
				JSON: spec.NotFound("File not found"),
			})
			return
		}
		// TODO: Handle the fact we might have started writing the response
		dReq.jsonErrorResponse(w, util.JSONResponse{
			Code: http.StatusNotFound,
			JSON: spec.NotFound("Failed to download: " + err.Error()),
		})
		return
	}

	if metadata == nil {
		dReq.jsonErrorResponse(w, util.JSONResponse{
			Code: http.StatusNotFound,
			JSON: spec.NotFound("File not found"),
		})
		return
	}

}

func (r *downloadRequest) jsonErrorResponse(w http.ResponseWriter, res util.JSONResponse) {
	// Marshal JSON response into raw bytes to send as the HTTP body
	resBytes, err := json.Marshal(res.JSON)
	if err != nil {
		r.Logger.WithError(err).Error("Failed to marshal JSONResponse")
		// this should never fail to be marshalled so drop err to the floor
		res = util.MessageResponse(http.StatusNotFound, "Download request failed: "+err.Error())
		resBytes, _ = json.Marshal(res.JSON)
	}

	// Set status code and write the body
	w.WriteHeader(res.Code)
	r.Logger.WithField("code", res.Code).Tracef("Responding (%d bytes)", len(resBytes))

	// we don't really care that much if we fail to write the error response
	w.Write(resBytes) // nolint: errcheck
}

// Validate validates the downloadRequest fields
func (r *downloadRequest) Validate() *util.JSONResponse {
	if !mediaIDRegex.MatchString(string(r.MediaMetadata.MediaID)) {
		return &util.JSONResponse{
			Code: http.StatusNotFound,
			JSON: spec.NotFound(fmt.Sprintf("mediaId must be a non-empty string using only characters in %v", mediaIDCharacters)),
		}
	}

	if r.IsThumbnailRequest {
		if r.ThumbnailSize.Width <= 0 || r.ThumbnailSize.Height <= 0 {
			return &util.JSONResponse{
				Code: http.StatusBadRequest,
				JSON: spec.Unknown("width and height must be greater than 0"),
			}
		}
		// Default method to scale if not set
		if r.ThumbnailSize.ResizeMethod == "" {
			r.ThumbnailSize.ResizeMethod = types.Scale
		}
		if r.ThumbnailSize.ResizeMethod != types.Crop && r.ThumbnailSize.ResizeMethod != types.Scale {
			return &util.JSONResponse{
				Code: http.StatusBadRequest,
				JSON: spec.Unknown("method must be one of crop or scale"),
			}
		}
	}
	return nil
}

func (r *downloadRequest) doDownload(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *config.FilesConfig,
	db storage.Database,
	activeThumbnailGeneration *types.ActiveThumbnailGeneration,
) (*types.MediaMetadata, error) {
	// check if we have a record of the media in our database
	mediaMetadata, err := db.GetMediaMetadata(ctx, r.MediaMetadata.MediaID)
	if err != nil {
		return nil, fmt.Errorf("db.GetMediaMetadata: %w", err)
	}
	if mediaMetadata == nil {
		// If we do not have a record and the origin is local, the file is not found
		return nil, nil
	} else {
		// If we have a record, we can respond from the local file
		r.MediaMetadata = mediaMetadata
	}
	return r.respondFromLocalFile(
		ctx, w, cfg.AbsBasePath, activeThumbnailGeneration,
		cfg.MaxThumbnailGenerators, db,
		cfg.DynamicThumbnails, cfg.ThumbnailSizes,
	)
}

// respondFromLocalFile reads a file from local storage and writes it to the http.ResponseWriter
// If no file was found then returns nil, nil
func (r *downloadRequest) respondFromLocalFile(
	ctx context.Context,
	w http.ResponseWriter,
	absBasePath config.Path,
	activeThumbnailGeneration *types.ActiveThumbnailGeneration,
	maxThumbnailGenerators int,
	db storage.Database,
	dynamicThumbnails bool,
	thumbnailSizes []config.ThumbnailSize,
) (*types.MediaMetadata, error) {
	filePath, err := utils.GetPathFromBase64Hash(r.MediaMetadata.Base64Hash, absBasePath)
	if err != nil {
		return nil, fmt.Errorf("utils.GetPathFromBase64Hash: %w", err)
	}
	file, err := os.Open(filePath)
	defer file.Close() // nolint: errcheck, staticcheck, megacheck
	if err != nil {
		return nil, fmt.Errorf("os.Open: %w", err)
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("file.Stat: %w", err)
	}

	if r.MediaMetadata.FileSizeBytes > 0 && int64(r.MediaMetadata.FileSizeBytes) != stat.Size() {
		r.Logger.WithFields(log.Fields{
			"fileSizeDatabase": r.MediaMetadata.FileSizeBytes,
			"fileSizeDisk":     stat.Size(),
		}).Warn("File size in database and on-disk differ.")
		return nil, errors.New("file size in database and on-disk differ")
	}

	var responseFile *os.File
	var responseMetadata *types.MediaMetadata
	if r.IsThumbnailRequest {
		thumbFile, thumbMetadata, resErr := r.getThumbnailFile(
			ctx, types.Path(filePath), activeThumbnailGeneration, maxThumbnailGenerators,
			db, dynamicThumbnails, thumbnailSizes,
		)
		if thumbFile != nil {
			defer thumbFile.Close() // nolint: errcheck
		}
		if resErr != nil {
			return nil, resErr
		}
		if thumbFile == nil {
			r.Logger.WithFields(log.Fields{
				"UploadName":    r.MediaMetadata.UploadName,
				"Base64Hash":    r.MediaMetadata.Base64Hash,
				"FileSizeBytes": r.MediaMetadata.FileSizeBytes,
				"ContentType":   r.MediaMetadata.ContentType,
			}).Trace("No good thumbnail found. Responding with original file.")
			responseFile = file
			responseMetadata = r.MediaMetadata
		} else {
			r.Logger.Trace("Responding with thumbnail")
			responseFile = thumbFile
			responseMetadata = thumbMetadata.MediaMetadata
		}
	} else {
		r.Logger.WithFields(log.Fields{
			"UploadName":    r.MediaMetadata.UploadName,
			"Base64Hash":    r.MediaMetadata.Base64Hash,
			"FileSizeBytes": r.MediaMetadata.FileSizeBytes,
			"ContentType":   r.MediaMetadata.ContentType,
		}).Trace("Responding with file")
		responseFile = file
		responseMetadata = r.MediaMetadata
		if err = r.addDownloadFilenameToHeaders(w, responseMetadata); err != nil {
			return nil, err
		}
	}

	w.Header().Set("Content-Type", string(responseMetadata.ContentType))
	w.Header().Set("Content-Length", strconv.FormatInt(int64(responseMetadata.FileSizeBytes), 10))
	contentSecurityPolicy := "default-src 'none';" +
		" script-src 'none';" +
		" plugin-types application/pdf;" +
		" style-src 'unsafe-inline';" +
		" object-src 'self';"

	if !r.multipartResponse {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		if _, err = io.Copy(w, responseFile); err != nil {
			return nil, fmt.Errorf("io.Copy: %w", err)
		}
	} else {
		var written int64
		written, err = multipartResponse(w, r, string(responseMetadata.ContentType), responseFile)
		if err != nil {
			return nil, err
		}
		responseMetadata.FileSizeBytes = types.FileSizeBytes(written)
	}
	return responseMetadata, nil
}

func multipartResponse(w http.ResponseWriter, r *downloadRequest, contentType string, responseFile io.Reader) (int64, error) {
	mw := multipart.NewWriter(w)
	// Update the header to be multipart/mixed; boundary=$randomBoundary
	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())
	w.Header().Del("Content-Length") // let Go handle the content length
	defer func() {
		if err := mw.Close(); err != nil {
			r.Logger.WithError(err).Error("Failed to close multipart writer")
		}
	}()

	// JSON object part
	jsonWriter, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type": {"application/json"},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create json writer: %w", err)
	}
	if _, err = jsonWriter.Write([]byte("{}")); err != nil {
		return 0, fmt.Errorf("failed to write to json writer: %w", err)
	}

	// media part
	mediaWriter, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type": {contentType},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create media writer: %w", err)
	}
	return io.Copy(mediaWriter, responseFile)
}

func (r *downloadRequest) addDownloadFilenameToHeaders(
	w http.ResponseWriter,
	responseMetadata *types.MediaMetadata,
) error {
	// If the requestor supplied a filename to name the download then
	// use that, otherwise use the filename from the response metadata.
	filename := string(responseMetadata.UploadName)
	if r.DownloadFilename != "" {
		filename = r.DownloadFilename
	}

	if len(filename) == 0 {
		w.Header().Set("Content-Disposition", contentDispositionFor(""))
		return nil
	}

	unescaped, err := url.PathUnescape(filename)
	if err != nil {
		return fmt.Errorf("url.PathUnescape: %w", err)
	}

	isASCII := true // Is the string ASCII or UTF-8?
	quote := ``     // Encloses the string (ASCII only)
	for i := 0; i < len(unescaped); i++ {
		if unescaped[i] > unicode.MaxASCII {
			isASCII = false
		}
		if unescaped[i] == 0x20 || unescaped[i] == 0x3B {
			// If the filename contains a space or a semicolon, which
			// are special characters in Content-Disposition
			quote = `"`
		}
	}

	// We don't necessarily want a full escape as the Content-Disposition
	// can take many of the characters that PathEscape would otherwise and
	// browser support for encoding is a bit wild, so we'll escape only
	// the characters that we know will mess up the parsing of the
	// Content-Disposition header elements themselves
	unescaped = strings.ReplaceAll(unescaped, `\`, `\\"`)
	unescaped = strings.ReplaceAll(unescaped, `"`, `\"`)

	disposition := contentDispositionFor(responseMetadata.ContentType)
	if isASCII {
		// For ASCII filenames, we should only quote the filename if
		// it needs to be done, e.g. it contains a space or a character
		// that would otherwise be parsed as a control character in the
		// Content-Disposition header
		w.Header().Set("Content-Disposition", fmt.Sprintf(
			`%s; filename=%s%s%s`,
			disposition, quote, unescaped, quote,
		))
	} else {
		// For UTF-8 filenames, we quote always, as that's the standard
		w.Header().Set("Content-Disposition", fmt.Sprintf(
			`%s; filename*=utf-8''%s`,
			disposition, url.QueryEscape(unescaped),
		))
	}

	return nil
}

// Note: Thumbnail generation may be ongoing asynchronously.
// If no thumbnail was found then returns nil, nil, nil
func (r *downloadRequest) getThumbnailFile(
	ctx context.Context,
	filePath types.Path,
	activeThumbnailGeneration *types.ActiveThumbnailGeneration,
	maxThumbnailGenerators int,
	db storage.Database,
	dynamicThumbnails bool,
	thumbnailSizes []config.ThumbnailSize,
) (*os.File, *types.ThumbnailMetadata, error) {
	var thumbnail *types.ThumbnailMetadata
	var err error

	if dynamicThumbnails {
		thumbnail, err = r.generateThumbnail(
			ctx, filePath, r.ThumbnailSize, activeThumbnailGeneration,
			maxThumbnailGenerators, db,
		)
		if err != nil {
			return nil, nil, err
		}
	}
	// If dynamicThumbnails is true but there are too many thumbnails being actively generated, we can fall back
	// to trying to use a pre-generated thumbnail
	if thumbnail == nil {
		var thumbnails []*types.ThumbnailMetadata
		thumbnails, err = db.GetThumbnails(ctx, r.MediaMetadata.MediaID)
		if err != nil {
			return nil, nil, fmt.Errorf("db.GetThumbnails: %w", err)
		}

		// If we get a thumbnailSize, a pre-generated thumbnail would be best but it is not yet generated.
		// If we get a thumbnail, we're done.
		var thumbnailSize *types.ThumbnailSize
		thumbnail, thumbnailSize = thumbnailer.SelectThumbnail(r.ThumbnailSize, thumbnails, thumbnailSizes)
		// If dynamicThumbnails is true and we are not over-loaded then we would have generated what was requested above.
		// So we don't try to generate a pre-generated thumbnail here.
		if thumbnailSize != nil && !dynamicThumbnails {
			r.Logger.WithFields(log.Fields{
				"Width":        thumbnailSize.Width,
				"Height":       thumbnailSize.Height,
				"ResizeMethod": thumbnailSize.ResizeMethod,
			}).Debug("Pre-generating thumbnail for immediate response.")
			thumbnail, err = r.generateThumbnail(
				ctx, filePath, *thumbnailSize, activeThumbnailGeneration,
				maxThumbnailGenerators, db,
			)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if thumbnail == nil {
		return nil, nil, nil
	}
	r.Logger = r.Logger.WithFields(log.Fields{
		"Width":         thumbnail.ThumbnailSize.Width,
		"Height":        thumbnail.ThumbnailSize.Height,
		"ResizeMethod":  thumbnail.ThumbnailSize.ResizeMethod,
		"FileSizeBytes": thumbnail.MediaMetadata.FileSizeBytes,
		"ContentType":   thumbnail.MediaMetadata.ContentType,
	})
	thumbPath := string(thumbnailer.GetThumbnailPath(filePath, *thumbnail.ThumbnailSize))
	thumbFile, err := os.Open(string(thumbPath))
	if err != nil {
		thumbFile.Close() // nolint: errcheck
		return nil, nil, fmt.Errorf("os.Open: %w", err)
	}
	thumbStat, err := thumbFile.Stat()
	if err != nil {
		thumbFile.Close() // nolint: errcheck
		return nil, nil, fmt.Errorf("thumbFile.Stat: %w", err)
	}
	if types.FileSizeBytes(thumbStat.Size()) != thumbnail.MediaMetadata.FileSizeBytes {
		thumbFile.Close() // nolint: errcheck
		return nil, nil, errors.New("thumbnail file sizes on disk and in database differ")
	}
	return thumbFile, thumbnail, nil
}

func (r *downloadRequest) generateThumbnail(
	ctx context.Context,
	filePath types.Path,
	thumbnailSize types.ThumbnailSize,
	activeThumbnailGeneration *types.ActiveThumbnailGeneration,
	maxThumbnailGenerators int,
	db storage.Database,
) (*types.ThumbnailMetadata, error) {
	r.Logger.WithFields(log.Fields{
		"Width":        thumbnailSize.Width,
		"Height":       thumbnailSize.Height,
		"ResizeMethod": thumbnailSize.ResizeMethod,
	})
	busy, err := thumbnailer.GenerateThumbnail(
		ctx, filePath, thumbnailSize, r.MediaMetadata,
		activeThumbnailGeneration, maxThumbnailGenerators, db, r.Logger,
	)
	if err != nil {
		return nil, fmt.Errorf("thumbnailer.GenerateThumbnail: %w", err)
	}
	if busy {
		return nil, nil
	}
	var thumbnail *types.ThumbnailMetadata
	thumbnail, err = db.GetThumbnail(ctx, r.MediaMetadata.MediaID,
		thumbnailSize.Width, thumbnailSize.Height, thumbnailSize.ResizeMethod,
	)
	if err != nil {
		return nil, fmt.Errorf("db.GetThumbnail: %w", err)
	}
	return thumbnail, nil
}

func (r *downloadRequest) GetContentLengthAndReader(contentLengthHeader string, reader io.ReadCloser, maxFileSizeBytes config.FileSizeBytes) (int64, io.Reader, error) {
	var contentLength int64

	if contentLengthHeader != "" {
		// A Content-Length header is provided. Let's try to parse it.
		parsedLength, parseErr := strconv.ParseInt(contentLengthHeader, 10, 64)
		if parseErr != nil {
			r.Logger.WithError(parseErr).Warn("Failed to parse content length")
			return 0, nil, fmt.Errorf("strconv.ParseInt: %w", parseErr)
		}
		if maxFileSizeBytes > 0 && parsedLength > int64(maxFileSizeBytes) {
			return 0, nil, fmt.Errorf(
				"remote file size (%d bytes) exceeds locally configured max media size (%d bytes)",
				parsedLength, maxFileSizeBytes,
			)
		}

		// We successfully parsed the Content-Length, so we'll return a limited
		// reader that restricts us to reading only up to this size.
		reader = io.NopCloser(io.LimitReader(reader, parsedLength))
		contentLength = parsedLength
	} else {
		// Content-Length header is missing. If we have a maximum file size
		// configured then we'll just make sure that the reader is limited to
		// that size. We'll return a zero content length, but that's OK, since
		// ultimately it will get rewritten later when the temp file is written
		// to disk.
		if maxFileSizeBytes > 0 {
			reader = io.NopCloser(io.LimitReader(reader, int64(maxFileSizeBytes)))
		}
		contentLength = 0
	}

	return contentLength, reader, nil
}

// mediaMeta contains information about a multipart media response.
// TODO: extend once something is defined.
type mediaMeta struct{}

func parseMultipartResponse(r *downloadRequest, resp *http.Response, maxFileSizeBytes config.FileSizeBytes) (int64, io.Reader, error) {
	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return 0, nil, err
	}
	if params["boundary"] == "" {
		return 0, nil, fmt.Errorf("no boundary header found on media %s ", r.MediaMetadata.MediaID)
	}
	mr := multipart.NewReader(resp.Body, params["boundary"])

	// Get the first, JSON, part
	p, err := mr.NextPart()
	if err != nil {
		return 0, nil, err
	}
	defer p.Close() // nolint: errcheck

	if p.Header.Get("Content-Type") != "application/json" {
		return 0, nil, fmt.Errorf("first part of the response must be application/json")
	}
	// Try to parse media meta information
	meta := mediaMeta{}
	if err = json.NewDecoder(p).Decode(&meta); err != nil {
		return 0, nil, err
	}
	defer p.Close() // nolint: errcheck

	// Get the actual media content
	p, err = mr.NextPart()
	if err != nil {
		return 0, nil, err
	}

	redirect := p.Header.Get("Location")
	if redirect != "" {
		return 0, nil, fmt.Errorf("Location header is not yet supported")
	}

	contentLength, reader, err := r.GetContentLengthAndReader(p.Header.Get("Content-Length"), p, maxFileSizeBytes)
	// For multipart requests, we need to get the Content-Type of the second part, which is the actual media
	r.MediaMetadata.ContentType = types.ContentType(p.Header.Get("Content-Type"))
	return contentLength, reader, err
}

// contentDispositionFor returns the Content-Disposition for a given
// content type.
func contentDispositionFor(contentType types.ContentType) string {
	if _, ok := allowInlineTypes[contentType]; ok {
		return "inline"
	}
	return "attachment"
}
