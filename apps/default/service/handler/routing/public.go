package routing

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/pitabwire/frame/v2"
	"github.com/pitabwire/util"
)

const (
	// AnonymousMediaPathPrefix is the URL prefix for unauthenticated access to PUBLIC media.
	// It must be mounted outside the authentication middleware.
	AnonymousMediaPathPrefix = "/v1/public/media/"

	// immutableCacheControl marks public media as immutable: a media id never changes bytes,
	// so browsers and CDNs may cache it for a year without revalidation.
	immutableCacheControl = "public, max-age=31536000, immutable"

	// transientCacheControl is used when the original stands in for a thumbnail that does not
	// exist yet; caches must come back soon so the real thumbnail is picked up once generated.
	transientCacheControl = "public, max-age=300"
)

// SetupPublicMediaRoutes builds the router for anonymous, cacheable access to PUBLIC media:
//
//	GET|HEAD /v1/public/media/{serverName}/{mediaId}
//	GET|HEAD /v1/public/media/{serverName}/{mediaId}/thumbnail?width=&height=&method=
//
// The only gate is that the media exists (not deleted) and has PUBLIC visibility; anything
// else answers 404 so private media cannot be enumerated.
func SetupPublicMediaRoutes(
	service *frame.Service,
	db storage.Database,
	provider storage.Provider,
	mediaService business.MediaService,
) *Router {
	cfg := service.Config().(*config.FilesConfig)
	router := NewRouter()
	v1 := router.PathPrefix(AnonymousMediaPathPrefix)

	h := &publicMediaHandler{cfg: cfg, db: db, provider: provider, mediaService: mediaService}
	methods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	v1.Handle("/{serverName}/{mediaId}", WrapHandlerInCORS(h.serve(false))).Methods(methods...)
	v1.Handle("/{serverName}/{mediaId}/thumbnail", WrapHandlerInCORS(h.serve(true))).Methods(methods...)

	return router
}

type publicMediaHandler struct {
	cfg          *config.FilesConfig
	db           storage.Database
	provider     storage.Provider
	mediaService business.MediaService
}

func (h *publicMediaHandler) serve(isThumbnail bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req = util.RequestWithLogging(req)
		ctx := req.Context()

		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")

		mediaID := pathVar(ctx, "mediaId")
		if !isValidMediaID(mediaID) {
			publicNotFound(w)
			return
		}

		mediaMetadata, err := h.db.GetMediaMetadata(ctx, types.MediaID(mediaID))
		if err != nil {
			util.Log(ctx).WithError(err).With("media_id", mediaID).Error("public media: metadata lookup failed")
			publicError(w, http.StatusInternalServerError)
			return
		}
		// Soft-deleted rows are filtered by the repository, so nil covers both unknown and deleted.
		if mediaMetadata == nil || !mediaMetadata.IsPublic {
			publicNotFound(w)
			return
		}

		target := mediaMetadata
		cacheControl := immutableCacheControl
		if isThumbnail {
			target, cacheControl, err = h.resolveThumbnail(ctx, req, mediaMetadata)
			if err != nil {
				switch {
				case errors.Is(err, business.ErrInvalidParameter):
					publicError(w, http.StatusBadRequest)
				case errors.Is(err, business.ErrThumbnailNotFound), errors.Is(err, business.ErrMediaNotFound):
					publicNotFound(w)
				default:
					util.Log(ctx).WithError(err).With("media_id", mediaID).Error("public media: thumbnail resolution failed")
					publicError(w, http.StatusInternalServerError)
				}
				return
			}
		}

		h.serveContent(w, req, target, cacheControl)
	})
}

// resolveThumbnail picks (or generates) the thumbnail for the request and returns the metadata
// to serve plus the Cache-Control to send. When no thumbnail exists for an image (the source is
// already no larger than requested, or pre-generation has not run yet) the original is served
// with a short cache lifetime instead of a 404.
func (h *publicMediaHandler) resolveThumbnail(ctx context.Context, req *http.Request, mediaMetadata *types.MediaMetadata) (*types.MediaMetadata, string, error) {
	size := parseThumbnailSize(req)
	thumb, err := h.mediaService.ResolveThumbnail(ctx, mediaMetadata, &business.DownloadRequest{
		MediaID:            mediaMetadata.MediaID,
		IsThumbnailRequest: true,
		ThumbnailSize:      size,
		Config:             h.cfg,
	})
	if err == nil {
		return thumb.MediaMetadata, immutableCacheControl, nil
	}
	if errors.Is(err, business.ErrThumbnailNotFound) && strings.HasPrefix(string(mediaMetadata.ContentType), "image/") {
		return mediaMetadata, transientCacheControl, nil
	}
	return nil, "", err
}

// parseThumbnailSize reads width/height/method from the query. Unlike the authenticated route,
// method defaults to scale so catalogue URLs can omit it.
func parseThumbnailSize(req *http.Request) *types.ThumbnailSize {
	q := req.URL.Query()
	width, err := strconv.Atoi(q.Get("width"))
	if err != nil {
		width = -1
	}
	height, err := strconv.Atoi(q.Get("height"))
	if err != nil {
		height = -1
	}
	method := strings.ToLower(q.Get("method"))
	if method == "" {
		method = types.Scale
	}
	return &types.ThumbnailSize{Width: width, Height: height, ResizeMethod: method}
}

// serveContent writes the object with CDN-friendly headers. Plaintext objects go through
// http.ServeContent (Range, If-None-Match, If-Modified-Since, HEAD); the rare encrypted public
// object is streamed whole.
func (h *publicMediaHandler) serveContent(w http.ResponseWriter, req *http.Request, meta *types.MediaMetadata, cacheControl string) {
	ctx := req.Context()
	logger := util.Log(ctx).With("media_id", meta.MediaID)

	contentType := string(meta.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	modTime := time.UnixMilli(int64(meta.CreationTimestamp)).UTC()
	etag := `"` + string(meta.Base64Hash) + `"`

	hdr := w.Header()
	hdr.Set("Content-Type", contentType)
	hdr.Set("ETag", etag)
	hdr.Set("Cache-Control", cacheControl)

	if meta.Encryption != nil {
		h.streamEncrypted(w, req, meta, modTime)
		return
	}

	key, err := utils.GetPathFromBase64Hash(meta.Base64Hash, h.cfg.AbsBasePath)
	if err != nil {
		logger.WithError(err).Error("public media: could not derive object path")
		publicError(w, http.StatusInternalServerError)
		return
	}

	rs, err := storage.NewBlobReadSeeker(ctx, h.provider, h.provider.GetBucket(meta.IsPublic), types.Path(key), int64(meta.FileSizeBytes))
	if err != nil {
		logger.WithError(err).Error("public media: could not open object")
		publicError(w, http.StatusInternalServerError)
		return
	}
	defer util.CloseAndLogOnError(ctx, rs)

	http.ServeContent(w, req, "", modTime, rs)
}

func (h *publicMediaHandler) streamEncrypted(w http.ResponseWriter, req *http.Request, meta *types.MediaMetadata, modTime time.Time) {
	ctx := req.Context()
	if match := req.Header.Get("If-None-Match"); match != "" && match == w.Header().Get("ETag") {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	// Decrypted streams cannot be seeked, so advertise no range support on this path.
	w.Header().Set("Accept-Ranges", "none")
	w.Header().Set("Last-Modified", modTime.Format(http.TimeFormat))
	w.Header().Set("Content-Length", strconv.FormatInt(int64(meta.FileSizeBytes), 10))
	if req.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	result, err := h.mediaService.DownloadFile(ctx, &business.DownloadRequest{MediaID: meta.MediaID, Config: h.cfg})
	if err != nil {
		util.Log(ctx).WithError(err).With("media_id", meta.MediaID).Error("public media: download failed")
		publicError(w, http.StatusInternalServerError)
		return
	}
	defer util.CloseAndLogOnError(ctx, result.FileData)

	w.WriteHeader(http.StatusOK)
	if _, err = io.Copy(w, result.FileData); err != nil {
		util.Log(ctx).WithError(err).With("media_id", meta.MediaID).Error("public media: failed to stream content")
	}
}

func pathVar(ctx context.Context, name string) string {
	v, _ := ctx.Value(ctxValueString(name)).(string)
	return v
}

func publicNotFound(w http.ResponseWriter) {
	publicError(w, http.StatusNotFound)
}

func publicError(w http.ResponseWriter, code int) {
	hdr := w.Header()
	hdr.Del("ETag")
	hdr.Set("Cache-Control", "no-store")
	hdr.Set("Content-Type", "text/plain; charset=utf-8")
	http.Error(w, http.StatusText(code), code)
}
