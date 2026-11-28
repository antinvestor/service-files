package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"buf.build/gen/go/antinvestor/files/connectrpc/go/v2/files/v1/filesv1connect"
	filesv1 "buf.build/gen/go/antinvestor/files/protocolbuffers/go/files/v1"
	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/authz"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/tests/testketo"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/pitabwire/frame/v3/frametests/deps/testpostgres"
	"github.com/pitabwire/frame/v3/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ServiceAccountHandlerTestSuite runs the FileServer against a real Keto so
// service-account authorisation is enforced end to end (the shared
// FileServerTestSuite runs with authorization disabled).
type ServiceAccountHandlerTestSuite struct {
	tests.BaseTestSuite
}

func TestServiceAccountHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAccountHandlerTestSuite))
}

func (suite *ServiceAccountHandlerTestSuite) SetupSuite() {
	suite.InitResourceFunc = func(_ context.Context) []definition.TestResource {
		pg := testpostgres.NewWithOpts("service_sa_handler",
			definition.WithUserName("test"),
			definition.WithEnableLogging(true))
		keto := testketo.NewWithOpts(definition.WithDependancies(pg))
		return []definition.TestResource{pg, keto}
	}
	suite.BaseTestSuite.SetupSuite()
}

func (suite *ServiceAccountHandlerTestSuite) setupKetoFileServer(
	t *testing.T, dep *definition.DependencyOption,
) (context.Context, security.Authorizer, *FileServer) {
	var ketoDep definition.DependancyConn
	for _, res := range suite.Resources() {
		if res.Name() == testketo.ImageName {
			ketoDep = res
		}
	}
	require.NotNil(t, ketoDep)
	writeURL, err := url.Parse(string(ketoDep.GetDS(t.Context())))
	require.NoError(t, err)
	readPort, err := ketoDep.PortMapping(t.Context(), "4466/tcp")
	require.NoError(t, err)
	t.Setenv("AUTHORIZATION_SERVICE_WRITE_URI", writeURL.Host)
	t.Setenv("AUTHORIZATION_SERVICE_READ_URI", writeURL.Hostname()+":"+readPort)

	ctx, svc, res := suite.CreateService(t, dep)
	cfg := svc.Config().(*config.FilesConfig)
	db := &connection.Database{
		WorkManager:             svc.WorkManager(),
		MediaRepository:         res.MediaRepository,
		MultipartUploadRepo:     res.MultipartUploadRepo,
		MultipartUploadPartRepo: res.MultipartUploadPartRepo,
		FileVersionRepo:         res.FileVersionRepo,
		RetentionPolicyRepo:     res.RetentionPolicyRepo,
		FileRetentionRepo:       res.FileRetentionRepo,
		StorageStatsRepo:        res.StorageStatsRepo,
	}
	storageProvider, err := provider.GetStorageProvider(ctx, cfg)
	require.NoError(t, err)

	auth := svc.SecurityManager().GetAuthorizer(ctx)
	fileServer := NewFileServer(svc, business.NewMediaService(db, storageProvider),
		authz.NewMiddleware(auth, db), db, storageProvider).(*FileServer)
	return ctx, auth, fileServer
}

const (
	saTenantID    = "sa-handler-tenant"
	saPartitionID = "sa-handler-partition"
)

func serviceAccountClaimsCtx(ctx context.Context, serviceName, profileID string) context.Context {
	authClaims := &security.AuthenticationClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: profileID},
		ProfileID:        profileID,
		TenantID:         saTenantID,
		PartitionID:      saPartitionID,
		ServiceName:      serviceName,
		Roles:            []string{"internal"},
	}
	return authClaims.ClaimsToContext(ctx)
}

func grantServiceFile(t *testing.T, auth security.Authorizer, permission, profileID string) {
	t.Helper()
	require.NoError(t, auth.WriteTuple(t.Context(), security.RelationTuple{
		Object:   security.ObjectRef{Namespace: authz.NamespaceServiceFile, ID: saTenantID + "/" + saPartitionID},
		Relation: "granted_" + permission,
		Subject:  security.SubjectRef{Namespace: authz.NamespaceProfile, ID: profileID},
	}))
}

func streamUpload(ctx context.Context, t *testing.T, fileHandler *FileServer, callerCtx func(context.Context) context.Context) error {
	t.Helper()
	mux := http.NewServeMux()
	Mount(mux, fileHandler)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(callerCtx(r.Context())))
	}))
	defer server.Close()

	client := filesv1connect.NewFilesServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL)))
	stream, err := client.UploadContent(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&filesv1.UploadContentRequest{Data: &filesv1.UploadContentRequest_Metadata{
		Metadata: &filesv1.UploadMetadata{Filename: "avatar.png", ContentType: "text/plain", TotalSize: 5},
	}}))
	require.NoError(t, stream.Send(&filesv1.UploadContentRequest{Data: &filesv1.UploadContentRequest_Chunk{Chunk: []byte("hello")}}))
	_, err = stream.CloseAndReceive()
	return err
}

// Service accounts carrying a service_name claim that are not in the built-in
// privileged policy are authorised by their service_file grants instead of
// being refused (service_authentication avatar sync, opportunities_matching CV
// upload, service_identity KYC documents, ...).
func (suite *ServiceAccountHandlerTestSuite) TestServiceAccountGrants() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, auth, fileHandler := suite.setupKetoFileServer(t, dep)

		t.Run("upload_refused_without_grant", func(t *testing.T) {
			svcCtx := serviceAccountClaimsCtx(ctx, "service_authentication", "sa-auth-nogrant")
			_, err := fileHandler.CreateContent(svcCtx, &filesv1.CreateContentRequest{})
			require.Error(t, err)
			assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

			err = streamUpload(ctx, t, fileHandler, func(c context.Context) context.Context {
				return serviceAccountClaimsCtx(c, "service_authentication", "sa-auth-nogrant")
			})
			require.Error(t, err)
			assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
		})

		t.Run("upload_allowed_with_content_upload_grant", func(t *testing.T) {
			grantServiceFile(t, auth, authz.PermissionContentUpload, "sa-auth-avatar")
			svcCtx := serviceAccountClaimsCtx(ctx, "service_authentication", "sa-auth-avatar")
			resp, err := fileHandler.CreateContent(svcCtx, &filesv1.CreateContentRequest{})
			require.NoError(t, err)
			assert.NotEmpty(t, resp.GetMediaId())

			err = streamUpload(ctx, t, fileHandler, func(c context.Context) context.Context {
				return serviceAccountClaimsCtx(c, "service_authentication", "sa-auth-avatar")
			})
			// Publishing the post-upload event may fail in this harness (see
			// Test_FileServer_UploadContentStream); authorisation must not.
			if err != nil {
				assert.NotEqual(t, connect.CodePermissionDenied, connect.CodeOf(err), err.Error())
			}
		})

		userFile := &types.MediaMetadata{
			MediaID: "sahandlercv0001", OwnerID: "sa-handler-end-user", UploadName: "cv.pdf",
			ContentType: "application/pdf", Base64Hash: "sa-handler-hash", FileSizeBytes: 5, ServerName: "server",
		}
		userCtx := (&security.AuthenticationClaims{
			RegisteredClaims: jwt.RegisteredClaims{Subject: "sa-handler-end-user"},
			TenantID:         saTenantID,
			PartitionID:      saPartitionID,
		}).ClaimsToContext(ctx)
		require.NoError(t, fileHandler.db.StoreMediaMetadata(userCtx, userFile))

		t.Run("view_refused_without_grant", func(t *testing.T) {
			svcCtx := serviceAccountClaimsCtx(ctx, "opportunities_matching", "sa-matching-nogrant")
			_, err := fileHandler.HeadContent(svcCtx, &filesv1.HeadContentRequest{MediaId: string(userFile.MediaID)})
			require.Error(t, err)
			assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
		})

		t.Run("view_allowed_with_content_view_grant", func(t *testing.T) {
			grantServiceFile(t, auth, authz.PermissionContentView, "sa-matching-view")
			svcCtx := serviceAccountClaimsCtx(ctx, "opportunities_matching", "sa-matching-view")
			resp, err := fileHandler.HeadContent(svcCtx, &filesv1.HeadContentRequest{MediaId: string(userFile.MediaID)})
			require.NoError(t, err)
			assert.Equal(t, string(userFile.MediaID), resp.GetMetadata().GetMediaId())
		})

		t.Run("ocr_policy_unchanged", func(t *testing.T) {
			svcCtx := internalServiceClaimsCtx(ctx, "service_ocr")
			_, err := fileHandler.CreateContent(svcCtx, &filesv1.CreateContentRequest{})
			require.NoError(t, err)
			_, err = fileHandler.HeadContent(svcCtx, &filesv1.HeadContentRequest{MediaId: string(userFile.MediaID)})
			require.NoError(t, err)
		})
	})
}
