package routing

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/v2/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PublicRoutingTestSuite struct {
	tests.BaseTestSuite
}

func TestPublicRoutingTestSuite(t *testing.T) {
	suite.Run(t, new(PublicRoutingTestSuite))
}

const publicTestServer = "example.com"

func publicPath(mediaID string) string {
	return AnonymousMediaPathPrefix + publicTestServer + "/" + mediaID
}

func uploadTestMedia(t *testing.T, ctx context.Context, svc business.MediaService, cfg *config.FilesConfig, mediaID, contentType string, content []byte, isPublic bool) {
	t.Helper()
	_, err := svc.UploadFile(ctx, &business.UploadRequest{
		OwnerID:       "@owner:example.com",
		MediaID:       types.MediaID(mediaID),
		UploadName:    "file",
		ContentType:   types.ContentType(contentType),
		FileSizeBytes: types.FileSizeBytes(len(content)),
		FileData:      bytes.NewReader(content),
		Config:        cfg,
		IsPublic:      isPublic,
	})
	require.NoError(t, err)
}

func encodeTestImage(t *testing.T, asPNG bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 200, 120))
	// Gradient so the encoders have real content to compress.
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = uint8(i)
		img.Pix[i+1] = uint8(i / 200)
		img.Pix[i+2] = 128
		img.Pix[i+3] = 255
	}
	var buf bytes.Buffer
	if asPNG {
		require.NoError(t, png.Encode(&buf, img))
	} else {
		require.NoError(t, jpeg.Encode(&buf, img, nil))
	}
	return buf.Bytes()
}

func (suite *PublicRoutingTestSuite) TestPublicMediaRoutes() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)
		cfg := svc.Config().(*config.FilesConfig)
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: res.MediaRepository,
		}
		storageProvider, err := provider.GetStorageProvider(ctx, cfg)
		require.NoError(t, err)
		mediaService := business.NewMediaService(db, storageProvider)
		router := SetupPublicMediaRoutes(svc, db, storageProvider, mediaService)

		publicContent := []byte("public-catalogue-bytes-0123456789")
		uploadTestMedia(t, ctx, mediaService, cfg, "pubMediaA", "text/plain", publicContent, true)
		uploadTestMedia(t, ctx, mediaService, cfg, "privMediaB", "text/plain", []byte("private-bytes"), false)
		uploadTestMedia(t, ctx, mediaService, cfg, "delMediaC", "text/plain", []byte("deleted-bytes"), true)
		require.NoError(t, db.DeleteMedia(ctx, "delMediaC"))

		publicMeta, err := db.GetMediaMetadata(ctx, "pubMediaA")
		require.NoError(t, err)
		require.NotNil(t, publicMeta)
		wantETag := `"` + string(publicMeta.Base64Hash) + `"`

		do := func(method, target string, headers map[string]string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(method, target, nil)
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			return rec
		}

		assertCacheHeaders := func(t *testing.T, rec *httptest.ResponseRecorder) {
			t.Helper()
			assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
			assert.Equal(t, "cross-origin", rec.Header().Get("Cross-Origin-Resource-Policy"))
			assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
			assert.Equal(t, "bytes", rec.Header().Get("Accept-Ranges"))
			assert.NotEmpty(t, rec.Header().Get("Last-Modified"))
			assert.Empty(t, rec.Header().Get("Vary"))
		}

		t.Run("public_media_served_with_cache_headers", func(t *testing.T) {
			rec := do(http.MethodGet, publicPath("pubMediaA"), nil)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, publicContent, rec.Body.Bytes())
			assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
			assert.Equal(t, "33", rec.Header().Get("Content-Length"))
			assert.Equal(t, wantETag, rec.Header().Get("ETag"))
			assertCacheHeaders(t, rec)
		})

		t.Run("if_none_match_returns_304", func(t *testing.T) {
			rec := do(http.MethodGet, publicPath("pubMediaA"), map[string]string{"If-None-Match": wantETag})
			assert.Equal(t, http.StatusNotModified, rec.Code)
			assert.Empty(t, rec.Body.Bytes())
			assert.Equal(t, wantETag, rec.Header().Get("ETag"))
		})

		t.Run("head_returns_headers_without_body", func(t *testing.T) {
			rec := do(http.MethodHead, publicPath("pubMediaA"), nil)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Body.Bytes())
			assert.Equal(t, "33", rec.Header().Get("Content-Length"))
			assert.Equal(t, wantETag, rec.Header().Get("ETag"))
			assertCacheHeaders(t, rec)
		})

		t.Run("range_request_returns_206", func(t *testing.T) {
			rec := do(http.MethodGet, publicPath("pubMediaA"), map[string]string{"Range": "bytes=7-15"})
			require.Equal(t, http.StatusPartialContent, rec.Code)
			assert.Equal(t, publicContent[7:16], rec.Body.Bytes())
			assert.Equal(t, "bytes 7-15/33", rec.Header().Get("Content-Range"))
			assert.Equal(t, "9", rec.Header().Get("Content-Length"))
		})

		t.Run("options_preflight_allowed", func(t *testing.T) {
			rec := do(http.MethodOptions, publicPath("pubMediaA"), map[string]string{"Access-Control-Request-Method": "GET"})
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
		})

		notFoundCases := []struct {
			name   string
			target string
		}{
			{name: "private_media_is_404", target: publicPath("privMediaB")},
			{name: "unknown_media_is_404", target: publicPath("nopeMediaZ")},
			{name: "deleted_media_is_404", target: publicPath("delMediaC")},
			{name: "invalid_media_id_is_404", target: publicPath("bad%20id")},
			{name: "private_thumbnail_is_404", target: publicPath("privMediaB") + "/thumbnail?width=32&height=32&method=crop"},
		}
		for _, tc := range notFoundCases {
			t.Run(tc.name, func(t *testing.T) {
				rec := do(http.MethodGet, tc.target, nil)
				assert.Equal(t, http.StatusNotFound, rec.Code)
				assert.Empty(t, rec.Header().Get("ETag"))
				assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			})
		}

		t.Run("method_not_routed", func(t *testing.T) {
			rec := do(http.MethodPost, publicPath("pubMediaA"), nil)
			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	})
}

func (suite *PublicRoutingTestSuite) TestPublicThumbnailRoutes() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)
		cfg := svc.Config().(*config.FilesConfig)
		db := &connection.Database{
			WorkManager:     svc.WorkManager(),
			MediaRepository: res.MediaRepository,
		}
		storageProvider, err := provider.GetStorageProvider(ctx, cfg)
		require.NoError(t, err)
		mediaService := business.NewMediaService(db, storageProvider)
		router := SetupPublicMediaRoutes(svc, db, storageProvider, mediaService)

		jpegBytes := encodeTestImage(t, false)
		pngBytes := encodeTestImage(t, true)
		uploadTestMedia(t, ctx, mediaService, cfg, "pubJpegA", "image/jpeg", jpegBytes, true)
		uploadTestMedia(t, ctx, mediaService, cfg, "pubPngB", "image/png", pngBytes, true)

		do := func(method, target string, headers map[string]string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(method, target, nil)
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			return rec
		}

		t.Run("configured_size_generated_on_demand", func(t *testing.T) {
			rec := do(http.MethodGet, publicPath("pubJpegA")+"/thumbnail?width=32&height=32&method=crop", nil)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
			assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
			assert.NotEmpty(t, rec.Header().Get("ETag"))
			assert.Equal(t, "bytes", rec.Header().Get("Accept-Ranges"))

			img, format, decodeErr := image.Decode(bytes.NewReader(rec.Body.Bytes()))
			require.NoError(t, decodeErr)
			assert.Equal(t, "jpeg", format)
			assert.Equal(t, 32, img.Bounds().Dx())
			assert.Equal(t, 32, img.Bounds().Dy())

			thumb, thumbErr := db.GetThumbnail(ctx, "pubJpegA", 32, 32, types.Crop)
			require.NoError(t, thumbErr)
			require.NotNil(t, thumb)
			assert.Equal(t, `"`+string(thumb.Base64Hash)+`"`, rec.Header().Get("ETag"))

			again := do(http.MethodGet, publicPath("pubJpegA")+"/thumbnail?width=32&height=32&method=crop",
				map[string]string{"If-None-Match": rec.Header().Get("ETag")})
			assert.Equal(t, http.StatusNotModified, again.Code)
		})

		t.Run("png_source_yields_png_thumbnail", func(t *testing.T) {
			rec := do(http.MethodGet, publicPath("pubPngB")+"/thumbnail?width=96&height=96&method=crop", nil)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
			_, format, decodeErr := image.Decode(bytes.NewReader(rec.Body.Bytes()))
			require.NoError(t, decodeErr)
			assert.Equal(t, "png", format)
		})

		t.Run("method_defaults_to_scale", func(t *testing.T) {
			rec := do(http.MethodGet, publicPath("pubJpegA")+"/thumbnail?width=640&height=480", nil)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
		})

		t.Run("head_thumbnail", func(t *testing.T) {
			rec := do(http.MethodHead, publicPath("pubJpegA")+"/thumbnail?width=32&height=32&method=crop", nil)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Body.Bytes())
			assert.NotEmpty(t, rec.Header().Get("Content-Length"))
		})

		t.Run("unconfigured_size_serves_closest_pregenerated", func(t *testing.T) {
			require.False(t, cfg.DynamicThumbnails)
			// No 50x50 is configured, so SelectThumbnail answers with a configured crop size
			// (the already generated 32x32 wins over the not-yet-generated 96x96).
			rec := do(http.MethodGet, publicPath("pubJpegA")+"/thumbnail?width=50&height=50&method=crop", nil)
			require.Equal(t, http.StatusOK, rec.Code)
			img, _, decodeErr := image.Decode(bytes.NewReader(rec.Body.Bytes()))
			require.NoError(t, decodeErr)
			assert.Contains(t, []int{32, 96}, img.Bounds().Dx())
			assert.Equal(t, img.Bounds().Dx(), img.Bounds().Dy())
		})

		t.Run("source_smaller_than_request_serves_original_briefly_cached", func(t *testing.T) {
			// 1024x1024 is not a configured size and dynamic thumbnails are off, so no thumbnail
			// can be produced; the original stands in with a short cache lifetime.
			rec := do(http.MethodGet, publicPath("pubPngB")+"/thumbnail?width=1024&height=1024&method=scale", nil)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
			assert.Equal(t, pngBytes, rec.Body.Bytes())
			assert.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"))
		})

		t.Run("invalid_dimensions_is_400", func(t *testing.T) {
			rec := do(http.MethodGet, publicPath("pubJpegA")+"/thumbnail?width=0&height=abc", nil)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	})
}
