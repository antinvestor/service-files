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
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/security"
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
	mediaService business.MediaService,
) util.JSONResponse {
	ctx := req.Context()
	logger := util.Log(ctx)

	// Get authenticated user
	authClaims := security.ClaimsFromContext(ctx)
	if authClaims == nil {
		return util.JSONResponse{
			Code: http.StatusUnauthorized,
			JSON: spec.Unknown("Unauthorised"),
		}
	}

	sub, err := authClaims.GetSubject()
	if err != nil {
		return util.JSONResponse{
			Code: http.StatusUnauthorized,
			JSON: spec.Unknown("Unauthorised"),
		}
	}

	ownerID := types.OwnerID(sub)

	// Parse query parameters
	queryStr := req.FormValue("query")
	pageStr := req.FormValue("page")
	limitStr := req.FormValue("limit")

	// Set default values
	page := int32(0)
	limit := int32(50)

	// Parse page
	if pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p >= 0 {
			page = int32(p)
		} else {
			logger.WithField("page", pageStr).Warn("Invalid page parameter, using default")
		}
	}

	// Parse limit
	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 32); err == nil && l > 0 && l <= 1000 {
			limit = int32(l)
		} else {
			logger.WithField("limit", limitStr).Warn("Invalid limit parameter, using default")
		}
	}

	// Log the search request
	logger.WithFields(map[string]interface{}{
		"owner_id": ownerID,
		"query":    queryStr,
		"page":     page,
		"limit":    limit,
	}).Info("Search request")

	// Create business request
	businessReq := &business.SearchRequest{
		OwnerID: ownerID,
		Query:   queryStr,
		Page:    page,
		Limit:   limit,
	}

	// Execute business logic
	result, err := mediaService.SearchMedia(ctx, businessReq)
	if err != nil {
		logger.WithError(err).Error("Search failed")
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: spec.Unknown("Search failed"),
		}
	}

	// Return search response
	return util.JSONResponse{
		Code: http.StatusOK,
		JSON: searchResponse{
			Results: result.Results,
			Count:   result.Count,
			Page:    result.Page,
			HasMore: result.HasMore,
		},
	}
}
