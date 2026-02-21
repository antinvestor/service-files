package authz

import (
	"context"
	"net/url"
	"testing"

	aconfig "github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/tests/testketo"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/frametests/deps/testpostgres"
	"github.com/pitabwire/frame/security"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AuthzMiddlewareTestSuite struct {
	tests.BaseTestSuite

	cfg        aconfig.FilesConfig
	service    *frame.Service
	authorizer security.Authorizer
	middleware Middleware
	mediaDB    *connection.Database
	mediaRepo  repository.MediaRepository
}

func TestAuthzMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(AuthzMiddlewareTestSuite))
}

func (s *AuthzMiddlewareTestSuite) SetupSuite() {
	s.InitResourceFunc = func(_ context.Context) []definition.TestResource {
		pg := testpostgres.NewWithOpts("service_authz",
			definition.WithUserName("ant"),
			definition.WithPassword("s3cr3t"),
			definition.WithEnableLogging(true))
		keto := testketo.NewWithOpts(definition.WithDependancies(pg))
		return []definition.TestResource{pg, keto}
	}

	s.BaseTestSuite.SetupSuite()

	ctx := s.T().Context()

	var pgDep definition.DependancyConn
	var ketoDep definition.DependancyConn
	for _, res := range s.Resources() {
		if res.GetDS(ctx).IsDB() {
			pgDep = res
		}
		if res.Name() == testketo.ImageName {
			ketoDep = res
		}
	}
	require.NotNil(s.T(), pgDep)
	require.NotNil(s.T(), ketoDep)

	dsn, _, err := pgDep.GetRandomisedDS(ctx, "authz")
	require.NoError(s.T(), err)

	writeURL, err := url.Parse(string(ketoDep.GetDS(ctx)))
	require.NoError(s.T(), err)
	readPort, err := ketoDep.PortMapping(ctx, "4466/tcp")
	require.NoError(s.T(), err)

	cfg := aconfig.FilesConfig{}
	cfg.DatabasePrimaryURL = []string{dsn.String()}
	cfg.DatabaseReplicaURL = []string{dsn.String()}
	cfg.DatabaseMigrate = true
	cfg.AuthorizationServiceWriteURI = writeURL.Host
	cfg.AuthorizationServiceReadURI = writeURL.Hostname() + ":" + readPort
	cfg.EnvStorageEncryptionPhrase = "0123456789abcdef0123456789abcdef"
	cfg.BasePath = aconfig.Path(s.T().TempDir())
	require.NoError(s.T(), cfg.Normalise())
	s.cfg = cfg

	ctx, svc := frame.NewServiceWithContext(ctx,
		frame.WithName("authz-tests"),
		frame.WithConfig(&s.cfg),
		frame.WithDatastore(),
		frametests.WithNoopDriver())

	dbManager := svc.DatastoreManager()
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	mediaRepo := repository.NewMediaRepository(ctx, dbPool, svc.WorkManager())

	svc.Init(ctx)
	require.NoError(s.T(), repository.Migrate(ctx, dbManager, "apps/default/migrations/0001"))
	require.NoError(s.T(), svc.Run(ctx, ""))

	mediaDB, err := connection.NewMediaDatabase(svc.WorkManager(), mediaRepo)
	require.NoError(s.T(), err)

	s.service = svc
	s.mediaRepo = mediaRepo
	s.mediaDB = mediaDB.(*connection.Database)
	s.authorizer = svc.SecurityManager().GetAuthorizer(ctx)
	s.middleware = NewMiddleware(s.authorizer, s.mediaDB)
}

func (s *AuthzMiddlewareTestSuite) TestCanViewAndEditAccess() {
	ctx := s.T().Context()

	media := &types.MediaMetadata{
		MediaID:       "media-1",
		OwnerID:       "owner-1",
		UploadName:    "file.txt",
		Base64Hash:    "hash1",
		FileSizeBytes: 10,
		ServerName:    "server",
	}
	require.NoError(s.T(), s.mediaDB.StoreMediaMetadata(ctx, media))

	cases := []struct {
		name        string
		userID      string
		relation    string
		checkFn     func(context.Context, string, string) error
		expectError bool
	}{
		{
			name:        "owner_can_view",
			userID:      "owner-1",
			checkFn:     s.middleware.CanViewFile,
			expectError: false,
		},
		{
			name:        "viewer_can_view",
			userID:      "viewer-1",
			relation:    RelationViewer,
			checkFn:     s.middleware.CanViewFile,
			expectError: false,
		},
		{
			name:        "editor_can_edit",
			userID:      "editor-1",
			relation:    RelationEditor,
			checkFn:     s.middleware.CanEditFile,
			expectError: false,
		},
		{
			name:        "viewer_cannot_edit",
			userID:      "viewer-2",
			relation:    RelationViewer,
			checkFn:     s.middleware.CanEditFile,
			expectError: true,
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			if tc.relation != "" {
				err := s.authorizer.WriteTuple(ctx, security.RelationTuple{
					Object:   security.ObjectRef{Namespace: NamespaceFile, ID: string(media.MediaID)},
					Relation: tc.relation,
					Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: tc.userID},
				})
				require.NoError(t, err)
			}

			err := tc.checkFn(ctx, tc.userID, string(media.MediaID))
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func (s *AuthzMiddlewareTestSuite) TestGrantAndRevokeAccess() {
	ctx := s.T().Context()

	media := &types.MediaMetadata{
		MediaID:       "media-2",
		OwnerID:       "owner-2",
		UploadName:    "file.txt",
		Base64Hash:    "hash2",
		FileSizeBytes: 10,
		ServerName:    "server",
	}
	require.NoError(s.T(), s.mediaDB.StoreMediaMetadata(ctx, media))

	cases := []struct {
		name     string
		role     string
		targetID string
	}{
		{
			name:     "grant_and_revoke_viewer",
			role:     "viewer",
			targetID: "user-1",
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			err := s.middleware.GrantFileAccess(ctx, "owner-2", string(media.MediaID), tc.targetID, tc.role)
			require.NoError(t, err)

			err = s.middleware.CanViewFile(ctx, tc.targetID, string(media.MediaID))
			require.NoError(t, err)

			err = s.middleware.RevokeFileAccess(ctx, "owner-2", string(media.MediaID), tc.targetID)
			require.NoError(t, err)

			err = s.middleware.CanViewFile(ctx, tc.targetID, string(media.MediaID))
			require.Error(t, err)
		})
	}
}

func (s *AuthzMiddlewareTestSuite) TestListUserShares() {
	ctx := s.T().Context()

	media := &types.MediaMetadata{
		MediaID:       "media-3",
		OwnerID:       "owner-3",
		UploadName:    "file.txt",
		Base64Hash:    "hash3",
		FileSizeBytes: 10,
		ServerName:    "server",
	}
	require.NoError(s.T(), s.mediaDB.StoreMediaMetadata(ctx, media))

	cases := []struct {
		name     string
		userID   string
		relation string
	}{
		{
			name:     "shares_listed",
			userID:   "user-shared",
			relation: RelationViewer,
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			err := s.authorizer.WriteTuple(ctx, security.RelationTuple{
				Object:   security.ObjectRef{Namespace: NamespaceFile, ID: string(media.MediaID)},
				Relation: tc.relation,
				Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: tc.userID},
			})
			require.NoError(t, err)

			files, err := s.middleware.ListUserShares(ctx, tc.userID)
			require.NoError(t, err)
			require.Contains(t, files, string(media.MediaID))
		})
	}
}

func (s *AuthzMiddlewareTestSuite) TestPermissionHelpersAndListings() {
	ctx := s.T().Context()

	media := &types.MediaMetadata{
		MediaID:       "media-4",
		OwnerID:       "owner-4",
		UploadName:    "file.txt",
		Base64Hash:    "hash4",
		FileSizeBytes: 10,
		ServerName:    "server",
	}
	require.NoError(s.T(), s.mediaDB.StoreMediaMetadata(ctx, media))

	testCases := []struct {
		name    string
		runTest func(t *testing.T)
	}{
		{
			name: "can_delete_as_owner",
			runTest: func(t *testing.T) {
				err := s.middleware.CanDeleteFile(ctx, "owner-4", string(media.MediaID))
				require.NoError(t, err)
			},
		},
		{
			name: "can_upload_requires_subject",
			runTest: func(t *testing.T) {
				err := s.middleware.CanUploadFile(ctx, "")
				require.Error(t, err)
				err = s.middleware.CanUploadFile(ctx, "owner-4")
				require.NoError(t, err)
			},
		},
		{
			name: "list_shared_with_owner",
			runTest: func(t *testing.T) {
				err := s.authorizer.WriteTuple(ctx, security.RelationTuple{
					Object:   security.ObjectRef{Namespace: NamespaceFile, ID: string(media.MediaID)},
					Relation: RelationViewer,
					Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: "viewer-4"},
				})
				require.NoError(t, err)

				_, err = s.middleware.ListSharedWith(ctx, "owner-4", string(media.MediaID))
				require.NoError(t, err)
			},
		},
		{
			name: "role_relation_mappings",
			runTest: func(t *testing.T) {
				require.Equal(t, RelationViewer, RoleToRelation("viewer"))
				require.Equal(t, RelationEditor, RoleToRelation("editor"))
				require.Equal(t, RelationUploader, RoleToRelation("uploader"))
				require.Equal(t, "", RoleToRelation("unknown"))

				require.Equal(t, PermissionView, RelationToPermission(RelationViewer))
				require.Equal(t, PermissionEdit, RelationToPermission(RelationEditor))
				require.Equal(t, PermissionUpload, RelationToPermission(RelationUploader))
				require.Equal(t, "", RelationToPermission("unknown"))
			},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, tc.runTest)
	}
}

func (s *AuthzMiddlewareTestSuite) TestOwnershipAndRoleValidationErrors() {
	ctx := s.T().Context()

	media := &types.MediaMetadata{
		MediaID:       "media-5",
		OwnerID:       "owner-5",
		UploadName:    "file.txt",
		Base64Hash:    "hash5",
		FileSizeBytes: 10,
		ServerName:    "server",
	}
	require.NoError(s.T(), s.mediaDB.StoreMediaMetadata(ctx, media))

	testCases := []struct {
		name      string
		run       func(t *testing.T) error
		expectErr error
	}{
		{
			name: "grant_denied_for_non_owner",
			run: func(_ *testing.T) error {
				return s.middleware.GrantFileAccess(ctx, "not-owner", "media-5", "viewer-5", "viewer")
			},
			expectErr: ErrNotOwner,
		},
		{
			name: "grant_rejects_invalid_role",
			run: func(_ *testing.T) error {
				return s.middleware.GrantFileAccess(ctx, "owner-5", "media-5", "viewer-5", "bad-role")
			},
			expectErr: ErrInvalidRelation,
		},
		{
			name: "revoke_denied_for_non_owner",
			run: func(_ *testing.T) error {
				return s.middleware.RevokeFileAccess(ctx, "not-owner", "media-5", "viewer-5")
			},
			expectErr: ErrNotOwner,
		},
		{
			name: "list_shared_with_denied_for_non_owner",
			run: func(_ *testing.T) error {
				_, err := s.middleware.ListSharedWith(ctx, "not-owner", "media-5")
				return err
			},
			expectErr: ErrNotOwner,
		},
		{
			name: "get_file_owner_not_found",
			run: func(_ *testing.T) error {
				_, err := s.middleware.GetFileOwner(ctx, "missing-media")
				return err
			},
			expectErr: ErrNotFound,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			err := tc.run(t)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.expectErr)
		})
	}
}
