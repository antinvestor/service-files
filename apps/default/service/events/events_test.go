package events_test

import (
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/events"
	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EventsTestSuite struct {
	tests.BaseTestSuite
}

func TestEventsTestSuite(t *testing.T) {
	suite.Run(t, new(EventsTestSuite))
}

func (suite *EventsTestSuite) TestAuditSaveEvent() {
	testCases := []struct {
		name      string
		payload   any
		shouldErr bool
	}{
		{
			name:      "invalid_payload_type",
			payload:   &models.MediaMetadata{},
			shouldErr: true,
		},
		{
			name: "valid_payload",
			payload: &models.MediaAudit{
				FileID: "file-1",
				Action: "download",
				Source: "test",
			},
			shouldErr: false,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		handler := &events.MediaAuditSaveEvent{AuditRepository: res.AuditRepository}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := handler.Validate(ctx, tc.payload)
				if tc.shouldErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)

				err = handler.Execute(ctx, tc.payload)
				require.NoError(t, err)

				audit := tc.payload.(*models.MediaAudit)
				require.NotEmpty(t, audit.GetID())
				saved, err := res.AuditRepository.GetByID(ctx, audit.GetID())
				require.NoError(t, err)
				require.NotNil(t, saved)
				assert.Equal(t, audit.FileID, saved.FileID)
				assert.Equal(t, audit.Action, saved.Action)
			})
		}
	})
}

func (suite *EventsTestSuite) TestMetadataSaveEvent() {
	testCases := []struct {
		name      string
		payload   any
		shouldErr bool
	}{
		{
			name:      "invalid_payload_type",
			payload:   &models.MediaAudit{},
			shouldErr: true,
		},
		{
			name: "valid_payload",
			payload: &models.MediaMetadata{
				OwnerID:    "owner-1",
				Name:       "file.jpg",
				Ext:        "jpg",
				Size:       123,
				OriginTs:   time.Now().Unix(),
				Public:     false,
				Mimetype:   "image/jpeg",
				Hash:       "hash-1",
				ServerName: "server-1",
			},
			shouldErr: false,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, _, res := suite.CreateService(t, dep)
		handler := &events.MediaMetadataSaveEvent{MediaRepository: res.MediaRepository}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := handler.Validate(ctx, tc.payload)
				if tc.shouldErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)

				err = handler.Execute(ctx, tc.payload)
				require.NoError(t, err)

				metadata := tc.payload.(*models.MediaMetadata)
				require.NotEmpty(t, metadata.GetID())
				saved, err := res.MediaRepository.GetByID(ctx, metadata.GetID())
				require.NoError(t, err)
				require.NotNil(t, saved)
				assert.Equal(t, metadata.Name, saved.Name)
				assert.Equal(t, metadata.Hash, saved.Hash)
			})
		}
	})
}

func (suite *EventsTestSuite) TestEventMetadata() {
	testCases := []struct {
		name      string
		eventName string
	}{
		{name: "audit_event_meta", eventName: "media.audit.save.event"},
		{name: "metadata_event_meta", eventName: "file.metadata.save.event"},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			switch tc.name {
			case "audit_event_meta":
				handler := &events.MediaAuditSaveEvent{}
				assert.Equal(t, tc.eventName, handler.Name())
				assert.IsType(t, models.MediaAudit{}, handler.PayloadType())
			case "metadata_event_meta":
				handler := &events.MediaMetadataSaveEvent{}
				assert.Equal(t, tc.eventName, handler.Name())
				assert.IsType(t, models.MediaMetadata{}, handler.PayloadType())
			}
		})
	}
}
