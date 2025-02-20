package storage_test

import (
	"context"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/events"
	"github.com/antinvestor/service-files/service/queue"
	"github.com/antinvestor/service-files/service/storage"
	"github.com/antinvestor/service-files/service/types"
	"github.com/antinvestor/service-files/testsutil"
	"github.com/pitabwire/frame"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func mustCreateService(t *testing.T) (context.Context, storage.Database, func()) {

	ctx := context.TODO()
	cfg, err := frame.ConfigFromEnv[config.FilesConfig]()
	assert.NoError(t, err)

	ctx, srv := frame.NewServiceWithContext(ctx, "profile tests",
		frame.Config(&cfg),
		frame.Datastore(ctx),
		frame.NoopDriver())

	thumbnailQueueHandler := queue.NewThumbnailQueueHandler(srv)

	thumbnailQueue := frame.RegisterSubscriber(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL, 2, &thumbnailQueueHandler)
	thumbnailQueuePublisher := frame.RegisterPublisher(cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL)

	serviceOptions := []frame.Option{
		thumbnailQueue, thumbnailQueuePublisher,
		frame.RegisterEvents(events.NewAuditSaveHandler(srv), events.NewMetadataSaveHandler(srv)),
	}

	srv.Init(serviceOptions...)

	err = srv.Run(ctx, "")
	assert.NoError(t, err)

	db, err := storage.NewMediaAPIDatasource(srv)
	if err != nil {
		t.Fatalf("mustCreateService returned %s", err)
	}
	return ctx, db, func() {
		srv.Stop(ctx)
	}
}
func TestMediaRepository(t *testing.T) {

	ctx, srv, cleanUpFunc, err := testsutil.GetTestService(context.TODO(), "media_repository")
	assert.NoErrorf(t, err, "failed to get test service")
	defer cleanUpFunc()

	db, err := storage.NewMediaAPIDatasource(srv)
	assert.NoErrorf(t, err, "failed to open media database")

	t.Run("can insert media & query media", func(t *testing.T) {
		metadata := &types.MediaMetadata{
			MediaID:       "testing",
			ContentType:   "image/png",
			FileSizeBytes: 10,
			UploadName:    "upload test",
			Base64Hash:    "dGVzdGluZw==",
			OwnerID:       "@alice:localhost",
		}
		if err := db.StoreMediaMetadata(ctx, metadata); err != nil {
			t.Fatalf("unable to store media metadata: %v", err)
		}
		// query by media id
		gotMetadata, err := db.GetMediaMetadata(ctx, metadata.MediaID)
		if err != nil {
			t.Fatalf("unable to query media metadata: %v", err)
		}
		if !reflect.DeepEqual(metadata, gotMetadata) {
			t.Fatalf("expected metadata %+v, got %v", metadata, gotMetadata)
		}
		// query by media hash
		gotMetadata, err = db.GetMediaMetadataByHash(ctx, metadata.OwnerID, metadata.Base64Hash)
		if err != nil {
			t.Fatalf("unable to query media metadata by hash: %v", err)
		}
		if !reflect.DeepEqual(metadata, gotMetadata) {
			t.Fatalf("expected metadata %+v, got %v", metadata, gotMetadata)
		}
	})

}

func TestThumbnailsStorage(t *testing.T) {
	ctx, srv, cleanUpFunc, err := testsutil.GetTestService(context.TODO(), "thumbnail_storage")
	assert.NoErrorf(t, err, "failed to get test service")
	defer cleanUpFunc()

	db, err := storage.NewMediaAPIDatasource(srv)
	assert.NoErrorf(t, err, "failed to open media database")

	t.Run("can insert thumbnails & query media", func(t *testing.T) {
		thumbnails := []*types.ThumbnailMetadata{
			{
				MediaMetadata: &types.MediaMetadata{
					MediaID:       "curerv4pf2t9jvceefgg",
					ParentID:      "testing",
					ContentType:   "image/png",
					FileSizeBytes: 6,
					ThumbnailSize: &types.ThumbnailSize{
						Width:        5,
						Height:       5,
						ResizeMethod: types.Crop,
					},
				},
			},
			{
				MediaMetadata: &types.MediaMetadata{
					MediaID:       "curerv4pf2t9jvceefgx",
					ParentID:      "testing",
					ContentType:   "image/png",
					FileSizeBytes: 7,
					ThumbnailSize: &types.ThumbnailSize{
						Width:        1,
						Height:       1,
						ResizeMethod: types.Scale,
					},
				},
			},
		}
		for i := range thumbnails {
			if err := db.StoreThumbnail(ctx, thumbnails[i]); err != nil {
				t.Fatalf("unable to store thumbnail metadata: %v", err)
			}
		}
		// query by single thumbnail
		gotMetadata, err := db.GetThumbnail(ctx,
			thumbnails[0].MediaMetadata.ParentID,
			thumbnails[0].ThumbnailSize.Width, thumbnails[0].ThumbnailSize.Height,
			thumbnails[0].ThumbnailSize.ResizeMethod,
		)
		if err != nil {
			t.Fatalf("unable to query thumbnail metadata: %v", err)
		}
		if !reflect.DeepEqual(thumbnails[0].MediaMetadata, gotMetadata.MediaMetadata) {
			t.Fatalf("expected metadata %+v, got %+v", thumbnails[0].MediaMetadata, gotMetadata.MediaMetadata)
		}
		if !reflect.DeepEqual(thumbnails[0].ThumbnailSize, gotMetadata.ThumbnailSize) {
			t.Fatalf("expected metadata %+v, got %+v", thumbnails[0].MediaMetadata, gotMetadata.MediaMetadata)
		}
		// query by all thumbnails
		gotMediadatas, err := db.GetThumbnails(ctx, thumbnails[0].MediaMetadata.ParentID)
		if err != nil {
			t.Fatalf("unable to query media metadata by hash: %v", err)
		}
		if len(gotMediadatas) != len(thumbnails) {
			t.Fatalf("expected %d stored thumbnail metadata, got %d", len(thumbnails), len(gotMediadatas))
		}
		for i := range gotMediadatas {
			// metadata may be returned in a different order than it was stored, perform a search
			metaDataMatches := func() bool {
				for _, t := range thumbnails {
					if reflect.DeepEqual(t.MediaMetadata, gotMediadatas[i].MediaMetadata) && reflect.DeepEqual(t.ThumbnailSize, gotMediadatas[i].ThumbnailSize) {
						return true
					}
				}
				return false
			}

			if !metaDataMatches() {
				t.Fatalf("expected metadata %+v, got %+v", thumbnails[i].MediaMetadata, gotMediadatas[i].MediaMetadata)

			}
		}
	})
}
