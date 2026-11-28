package authz

import (
	"context"
	"net/url"
	"testing"

	aconfig "github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/tests/testketo"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pitabwire/frame/v3"
	"github.com/pitabwire/frame/v3/datastore"
	"github.com/pitabwire/frame/v3/frametests"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/pitabwire/frame/v3/frametests/deps/testpostgres"
	"github.com/pitabwire/frame/v3/security"
	"github.com/pitabwire/frame/v3/security/authorizer"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AuthzMiddlewareTestSuite struct {
	tests.BaseTestSuite

	cfg        aconfig.FilesConfig
	service    *frame.Service
	authorizer security.Authorizer
	middleware Middleware
	mediaDB    storage.Database
	mediaRepo  repository.MediaRepository
}

func TestAuthzMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(AuthzMiddlewareTestSuite))
}

func (s *AuthzMiddlewareTestSuite) SetupSuite() {
	s.InitResourceFunc = func(_ context.Context) []definition.TestResource {
		pg := testpostgres.NewWithOpts("service_authz",
			definition.WithUserName("ant"),
			definition.WithCredential("s3cr3t"),
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

	mediaDB, err := connection.NewMediaDatabase(
		svc.WorkManager(),
		mediaRepo,
		repository.NewMultipartUploadRepository(ctx, dbPool, svc.WorkManager()),
		repository.NewMultipartUploadPartRepository(ctx, dbPool, svc.WorkManager()),
		repository.NewFileVersionRepository(ctx, dbPool, svc.WorkManager()),
		repository.NewRetentionPolicyRepository(ctx, dbPool, svc.WorkManager()),
		repository.NewFileRetentionRepository(ctx, dbPool, svc.WorkManager()),
		repository.NewStorageStatsRepository(ctx, dbPool, svc.WorkManager()),
	)
	require.NoError(s.T(), err)

	s.service = svc
	s.mediaRepo = mediaRepo
	s.mediaDB = mediaDB
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

func internalServiceClaimsCtx(ctx context.Context, serviceName string) context.Context {
	authClaims := &security.AuthenticationClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: serviceName},
		ServiceName:      serviceName,
		Roles:            []string{"internal"},
	}
	return authClaims.ClaimsToContext(ctx)
}

func (s *AuthzMiddlewareTestSuite) TestInternalServiceAccess() {
	ctx := s.T().Context()

	media := &types.MediaMetadata{
		MediaID:       "media-svc-1",
		OwnerID:       "owner-svc-1",
		UploadName:    "file.txt",
		Base64Hash:    "hashsvc1",
		FileSizeBytes: 10,
		ServerName:    "server",
	}
	require.NoError(s.T(), s.mediaDB.StoreMediaMetadata(ctx, media))

	cases := []struct {
		name        string
		serviceName string
		checkFn     func(context.Context) error
		expectError bool
	}{
		{
			name:        "ocr_service_can_view",
			serviceName: "service_ocr",
			checkFn: func(svcCtx context.Context) error {
				return s.middleware.CanViewFile(svcCtx, "service_ocr", string(media.MediaID))
			},
			expectError: false,
		},
		{
			name:        "ocr_service_can_upload",
			serviceName: "service_ocr",
			checkFn: func(svcCtx context.Context) error {
				return s.middleware.CanUploadFile(svcCtx, "service_ocr")
			},
			expectError: false,
		},
		{
			name:        "property_service_can_view",
			serviceName: "service_property",
			checkFn: func(svcCtx context.Context) error {
				return s.middleware.CanViewFile(svcCtx, "service_property", string(media.MediaID))
			},
			expectError: false,
		},
		{
			name:        "property_service_cannot_write",
			serviceName: "service_property",
			checkFn: func(svcCtx context.Context) error {
				return s.middleware.CanUploadFile(svcCtx, "service_property")
			},
			expectError: true,
		},
		{
			name:        "unknown_service_denied_view",
			serviceName: "service_unknown",
			checkFn: func(svcCtx context.Context) error {
				return s.middleware.CanViewFile(svcCtx, "service_unknown", string(media.MediaID))
			},
			expectError: true,
		},
		{
			name:        "regular_user_unaffected",
			serviceName: "",
			checkFn: func(_ context.Context) error {
				userCtx := ctx
				authClaims := &security.AuthenticationClaims{
					RegisteredClaims: jwt.RegisteredClaims{Subject: "regular-user"},
					Roles:            []string{"user"},
				}
				userCtx = authClaims.ClaimsToContext(userCtx)
				// Regular user is not owner and has no Keto relation — should be denied
				return s.middleware.CanViewFile(userCtx, "regular-user", string(media.MediaID))
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			svcCtx := ctx
			if tc.serviceName != "" {
				svcCtx = internalServiceClaimsCtx(ctx, tc.serviceName)
			}
			err := tc.checkFn(svcCtx)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func serviceAccountCtx(ctx context.Context, serviceName, profileID, tenantID, partitionID string) context.Context {
	authClaims := &security.AuthenticationClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: profileID},
		ProfileID:        profileID,
		TenantID:         tenantID,
		PartitionID:      partitionID,
		ServiceName:      serviceName,
		Roles:            []string{"internal"},
	}
	return authClaims.ClaimsToContext(ctx)
}

func userCtx(ctx context.Context, profileID, tenantID, partitionID string) context.Context {
	authClaims := &security.AuthenticationClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: profileID},
		ProfileID:        profileID,
		TenantID:         tenantID,
		PartitionID:      partitionID,
		Roles:            []string{"user"},
	}
	return authClaims.ClaimsToContext(ctx)
}

const (
	tenantA, partitionA = "tenant-sa-a", "partition-sa-a"
	tenantB, partitionB = "tenant-sa-b", "partition-sa-b"
)

// grantServicePermission writes the tuple tenancy materialises for a service
// account grant: service_file:<tenantA>/<partitionA>#granted_<permission>.
func (s *AuthzMiddlewareTestSuite) grantServicePermission(t *testing.T, permission, profileID string) {
	t.Helper()
	require.NoError(t, s.authorizer.WriteTuple(t.Context(), security.RelationTuple{
		Object:   security.ObjectRef{Namespace: NamespaceServiceFile, ID: tenantA + "/" + partitionA},
		Relation: "granted_" + permission,
		Subject:  security.SubjectRef{Namespace: NamespaceProfile, ID: profileID},
	}))
}

// TestServiceAccountFunctionalGrants covers service callers that are not in
// the built-in privileged policy (service_authentication, opportunities_matching,
// service_identity, ...): they are authorised by their Keto grants on the
// service_file namespace for the partition concerned, not refused outright.
func (s *AuthzMiddlewareTestSuite) TestServiceAccountFunctionalGrants() {
	ctx := s.T().Context()

	// Files owned by an end user, stored in two different partitions.
	userFileA := &types.MediaMetadata{
		MediaID: "media-sa-user-a", OwnerID: "end-user-a", UploadName: "cv.pdf",
		Base64Hash: "hash-sa-a", FileSizeBytes: 10, ServerName: "server",
	}
	require.NoError(s.T(), s.mediaDB.StoreMediaMetadata(userCtx(ctx, "end-user-a", tenantA, partitionA), userFileA))
	userFileB := &types.MediaMetadata{
		MediaID: "media-sa-user-b", OwnerID: "end-user-b", UploadName: "cv.pdf",
		Base64Hash: "hash-sa-b", FileSizeBytes: 10, ServerName: "server",
	}
	require.NoError(s.T(), s.mediaDB.StoreMediaMetadata(userCtx(ctx, "end-user-b", tenantB, partitionB), userFileB))

	stored, err := s.mediaDB.GetMediaMetadata(ctx, userFileA.MediaID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), tenantA, stored.TenantID)
	require.Equal(s.T(), partitionA, stored.PartitionID)

	s.Run("service_without_grant_cannot_upload", func() {
		svcCtx := serviceAccountCtx(ctx, "service_authentication", "sa-auth-nogrant", tenantA, partitionA)
		err := s.middleware.CanUploadFile(svcCtx, "sa-auth-nogrant")
		require.Error(s.T(), err)
		require.ErrorIs(s.T(), err, authorizer.ErrPermissionDenied)
	})

	s.Run("service_with_content_upload_grant_can_upload", func() {
		s.grantServicePermission(s.T(), PermissionContentUpload, "sa-auth-upload")
		svcCtx := serviceAccountCtx(ctx, "service_authentication", "sa-auth-upload", tenantA, partitionA)
		require.NoError(s.T(), s.middleware.CanUploadFile(svcCtx, "sa-auth-upload"))
	})

	s.Run("upload_grant_is_partition_scoped", func() {
		s.grantServicePermission(s.T(), PermissionContentUpload, "sa-scoped-upload")
		svcCtx := serviceAccountCtx(ctx, "opportunities_matching", "sa-scoped-upload", tenantB, partitionB)
		require.Error(s.T(), s.middleware.CanUploadFile(svcCtx, "sa-scoped-upload"))
	})

	s.Run("view_grant_does_not_imply_upload", func() {
		s.grantServicePermission(s.T(), PermissionContentView, "sa-view-only")
		svcCtx := serviceAccountCtx(ctx, "service_identity", "sa-view-only", tenantA, partitionA)
		require.Error(s.T(), s.middleware.CanUploadFile(svcCtx, "sa-view-only"))
	})

	s.Run("service_without_grant_cannot_view_user_file", func() {
		svcCtx := serviceAccountCtx(ctx, "opportunities_matching", "sa-match-nogrant", tenantA, partitionA)
		err := s.middleware.CanViewFile(svcCtx, "sa-match-nogrant", string(userFileA.MediaID))
		require.ErrorIs(s.T(), err, authorizer.ErrPermissionDenied)
	})

	s.Run("service_with_content_view_grant_can_view_file_in_partition", func() {
		s.grantServicePermission(s.T(), PermissionContentView, "sa-match-view")
		svcCtx := serviceAccountCtx(ctx, "opportunities_matching", "sa-match-view", tenantA, partitionA)
		require.NoError(s.T(), s.middleware.CanViewFile(svcCtx, "sa-match-view", string(userFileA.MediaID)))
	})

	s.Run("view_grant_does_not_reach_other_partitions_files", func() {
		s.grantServicePermission(s.T(), PermissionContentView, "sa-match-view-a")
		// Claims name partition B, but the grant only covers A and the
		// file lives in B: the file's own partition is what is checked.
		svcCtx := serviceAccountCtx(ctx, "opportunities_matching", "sa-match-view-a", tenantB, partitionB)
		require.Error(s.T(), s.middleware.CanViewFile(svcCtx, "sa-match-view-a", string(userFileB.MediaID)))
	})

	s.Run("view_grant_on_claims_partition_does_not_reach_file_in_other_partition", func() {
		// Claims and grant both name partition A; the file lives in B. Only
		// checking the file's own partition denies this.
		s.grantServicePermission(s.T(), PermissionContentView, "sa-claims-a-file-b")
		svcCtx := serviceAccountCtx(ctx, "opportunities_matching", "sa-claims-a-file-b", tenantA, partitionA)
		err := s.middleware.CanViewFile(svcCtx, "sa-claims-a-file-b", string(userFileB.MediaID))
		require.ErrorIs(s.T(), err, authorizer.ErrPermissionDenied)
		require.NotErrorIs(s.T(), err, ErrNotFound)
	})

	s.Run("privileged_policy_scope_gap_falls_back_to_grants", func() {
		// service_property's built-in policy is read-only. Write operations
		// are not hard-denied: like any service they need an explicit grant.
		denyCtx := serviceAccountCtx(ctx, "service_property", "sa-property-nogrant", tenantA, partitionA)
		require.ErrorIs(s.T(), s.middleware.CanEditFile(denyCtx, "sa-property-nogrant", string(userFileA.MediaID)),
			authorizer.ErrPermissionDenied)
		require.ErrorIs(s.T(), s.middleware.CanUploadFile(denyCtx, "sa-property-nogrant"), authorizer.ErrPermissionDenied)

		s.grantServicePermission(s.T(), PermissionContentManage, "sa-property-manage")
		s.grantServicePermission(s.T(), PermissionContentUpload, "sa-property-manage")
		allowCtx := serviceAccountCtx(ctx, "service_property", "sa-property-manage", tenantA, partitionA)
		require.NoError(s.T(), s.middleware.CanEditFile(allowCtx, "sa-property-manage", string(userFileA.MediaID)))
		require.NoError(s.T(), s.middleware.CanUploadFile(allowCtx, "sa-property-manage"))
		// Its built-in read access is unchanged and needs no grant.
		require.NoError(s.T(), s.middleware.CanViewFile(denyCtx, "sa-property-nogrant", string(userFileB.MediaID)))
	})

	s.Run("view_grant_does_not_imply_edit_or_delete", func() {
		s.grantServicePermission(s.T(), PermissionContentView, "sa-view-noedit")
		svcCtx := serviceAccountCtx(ctx, "service_identity", "sa-view-noedit", tenantA, partitionA)
		require.Error(s.T(), s.middleware.CanEditFile(svcCtx, "sa-view-noedit", string(userFileA.MediaID)))
		require.Error(s.T(), s.middleware.CanDeleteFile(svcCtx, "sa-view-noedit", string(userFileA.MediaID)))
	})

	s.Run("manage_and_delete_grants_allow_edit_and_delete", func() {
		s.grantServicePermission(s.T(), PermissionContentManage, "sa-manage")
		s.grantServicePermission(s.T(), PermissionContentDelete, "sa-manage")
		svcCtx := serviceAccountCtx(ctx, "service_imports", "sa-manage", tenantA, partitionA)
		require.NoError(s.T(), s.middleware.CanEditFile(svcCtx, "sa-manage", string(userFileA.MediaID)))
		require.NoError(s.T(), s.middleware.CanDeleteFile(svcCtx, "sa-manage", string(userFileA.MediaID)))
	})

	s.Run("service_owning_file_can_view_without_grant", func() {
		owned := &types.MediaMetadata{
			MediaID: "media-sa-owned", OwnerID: "sa-avatar-owner", UploadName: "avatar.png",
			Base64Hash: "hash-sa-owned", FileSizeBytes: 10, ServerName: "server",
		}
		svcCtx := serviceAccountCtx(ctx, "service_authentication", "sa-avatar-owner", tenantA, partitionA)
		require.NoError(s.T(), s.mediaDB.StoreMediaMetadata(svcCtx, owned))
		require.NoError(s.T(), s.middleware.CanViewFile(svcCtx, "sa-avatar-owner", string(owned.MediaID)))
	})

	s.Run("regular_user_upload_unchanged", func() {
		require.NoError(s.T(), s.middleware.CanUploadFile(userCtx(ctx, "plain-user", tenantA, partitionA), "plain-user"))
	})

	s.Run("regular_user_does_not_get_service_fallback", func() {
		// A user holding a service_file member/grant tuple must still be
		// subject to per-file checks for other people's files.
		s.grantServicePermission(s.T(), PermissionContentView, "plain-user-granted")
		uCtx := userCtx(ctx, "plain-user-granted", tenantA, partitionA)
		require.Error(s.T(), s.middleware.CanViewFile(uCtx, "plain-user-granted", string(userFileA.MediaID)))
	})
}
