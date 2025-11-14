// Copyright 2022 The Matrix.org Foundation C.I.C.
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

package connection

import (
	"context"
	"errors"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/workerpool"
	"gorm.io/gorm"
)

type Database struct {
	workManager     workerpool.Manager
	mediaRepository repository.MediaRepository
}

// StoreMediaMetadata inserts the metadata about the uploaded media into the database.
// Returns an error if the combination of MediaID and Origin are not unique in the table.
func (d *Database) StoreMediaMetadata(ctx context.Context, mediaMetadata *types.MediaMetadata) error {
	media := models.MediaMetadata{}
	media.Fill(mediaMetadata)
	return d.mediaRepository.Create(ctx, &media)
}

// GetMediaMetadata returns metadata about media stored on this server.
// The media could have been uploaded to this server or fetched from another server and cached here.
// Returns nil metadata if there is no metadata associated with this media.
func (d *Database) GetMediaMetadata(ctx context.Context, mediaID types.MediaID) (*types.MediaMetadata, error) {
	mediaMetadata, err := d.mediaRepository.GetByID(ctx, string(mediaID))
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return mediaMetadata.ToApi(), err
}

// GetMediaMetadataByHash returns metadata about media stored on this server.
// The media could have been uploaded to this server or fetched from another server and cached here.
// Returns nil metadata if there is no metadata associated with this media.
func (d *Database) GetMediaMetadataByHash(ctx context.Context, ownerId types.OwnerID, mediaHash types.Base64Hash) (*types.MediaMetadata, error) {
	mediaMetadata, err := d.mediaRepository.GetByHash(ctx, ownerId, mediaHash)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return mediaMetadata.ToApi(), err
}

func (d *Database) Search(ctx context.Context, query *data.SearchQuery) (workerpool.JobResultPipe[*types.MediaMetadata], error) {

	jobResult := workerpool.NewJob(func(ctx context.Context, result workerpool.JobResultPipe[*types.MediaMetadata]) error {

		metadataResult, err := d.mediaRepository.Search(ctx, query)

		if err != nil {
			return err
		}

		for {

			res, ok := metadataResult.ReadResult(ctx)
			if !ok {
				return nil
			}

			if res.IsError() {
				return res.Error()
			}

			for _, mediaMetadata := range res.Item() {
				err = result.WriteResult(ctx, mediaMetadata.ToApi())
				if err != nil {
					return err
				}
			}
		}
	})

	err := workerpool.SubmitJob(ctx, d.workManager, jobResult)
	if err != nil {
		return nil, err
	}

	return jobResult, nil
}

// StoreThumbnail inserts the metadata about the thumbnail into the database.
// Returns an error if the combination of MediaID and Origin are not unique in the table.
func (d *Database) StoreThumbnail(ctx context.Context, thumbnailMetadata *types.ThumbnailMetadata) error {
	return d.StoreMediaMetadata(ctx, thumbnailMetadata.MediaMetadata)
}

// GetThumbnail returns metadata about a specific thumbnail.
// The media could have been uploaded to this server or fetched from another server and cached here.
// Returns nil metadata if there is no metadata associated with this thumbnail.
func (d *Database) GetThumbnail(ctx context.Context, mediaID types.MediaID, width, height int, resizeMethod string) (*types.ThumbnailMetadata, error) {

	thumbnailSize := types.ThumbnailSize{
		Width:        width,
		Height:       height,
		ResizeMethod: resizeMethod,
	}
	mediaMetadata, err := d.mediaRepository.GetByParentIDAndThumbnailSize(ctx, mediaID, &thumbnailSize)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	apiMM := mediaMetadata.ToApi()

	return &types.ThumbnailMetadata{
		MediaMetadata: apiMM,
	}, nil
}

// GetThumbnails returns metadata about all thumbnails for a specific media stored on this server.
// The media could have been uploaded to this server or fetched from another server and cached here.
// Returns nil metadata if there are no thumbnails associated with this media.
func (d *Database) GetThumbnails(ctx context.Context, mediaID types.MediaID) ([]*types.ThumbnailMetadata, error) {
	metadatas, err := d.mediaRepository.GetByParentID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	apiTMM := make([]*types.ThumbnailMetadata, len(metadatas))
	for i, mm := range metadatas {
		apiTMM[i] = &types.ThumbnailMetadata{
			MediaMetadata: mm.ToApi(),
		}
	}

	return apiTMM, err
}
