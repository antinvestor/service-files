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
	"net/http"
	"strconv"

	"github.com/antinvestor/gomatrixserverlib/spec"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/util"
)

// searchResponse represents the response structure for search results
type searchResponse struct {
	Results []*types.MediaMetadata `json:"results"`
	Count   int                    `json:"total"`
	Page    int                    `json:"page"`
	HasMore bool                   `json:"has_more"`
}

// Search implements GET /search
// This endpoint allows searching for media files with query and pagination support
func Search(
	req *http.Request,
	service *frame.Service,
	db storage.Database,
) util.JSONResponse {
	ctx := req.Context()
	logger := util.Log(ctx)

	// Parse query parameters
	queryStr := ""
	pageStr := ""
	limitStr := ""

	ownerID := ""
	claims := frame.ClaimsFromContext(ctx)
	if claims != nil {
		ownerID, _ = claims.GetSubject()
	}

	searchProperties := map[string]any{
		"owner_id": ownerID,
	}

	for k, v := range req.URL.Query() {

		if k == "q" {
			queryStr = v[0]
		} else if k == "page" {
			pageStr = v[0]
		} else if k == "limit" {
			limitStr = v[0]
		} else {

			searchProperties[k] = v[0]
		}
	}

	// Parse pagination parameters
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		page = 0
	}
	count, err := strconv.Atoi(limitStr)
	if err != nil {
		count = 20
	}

	query, err := datastore.NewSearchQuery(
		ctx,
		queryStr, searchProperties,
		page, count,
	)
	if err != nil {
		logger.WithError(err).Error("Failed to create search query")
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: spec.InternalServerError{},
		}
	}

	// Convert models to API types
	modelResults, err := db.Search(ctx, query)
	if err != nil {
		logger.WithError(err).Error("Failed to execute search by owner ID")
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: spec.InternalServerError{},
		}
	}

	var finalResultList []*types.MediaMetadata
	for {

		result, ok := modelResults.ReadResult(ctx)
		if !ok {
			break
		}

		if result.IsError() {
			logger.WithError(result.Error()).Error("Failed to read search finalResultList")
			return util.JSONResponse{
				Code: http.StatusInternalServerError,
				JSON: spec.InternalServerError{},
			}
		}

		finalResultList = append(finalResultList, result.Item())
	}

	// Calculate pagination info
	hasMore := len(finalResultList) == count

	// Build response
	response := searchResponse{
		Results: finalResultList,
		Count:   len(finalResultList),
		Page:    page,
		HasMore: hasMore,
	}

	logger.WithField("results_count", len(finalResultList)).Debug("Search completed successfully")

	return util.JSONResponse{
		Code: http.StatusOK,
		JSON: response,
	}
}
