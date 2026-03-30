package tests

import (
	"context"
	"errors"
	"testing"

	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilepb "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"buf.build/gen/go/antinvestor/tenancy/connectrpc/go/tenancy/v1/tenancyv1connect"
	tenancypb "buf.build/gen/go/antinvestor/tenancy/protocolbuffers/go/tenancy/v1"
	"connectrpc.com/connect"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/frametests/deps/testpostgres"
	"github.com/pitabwire/util"
)

const (
	DefaultRandomStringLength = 8
)

// --- stub ProfileServiceClient ---

type stubProfileServiceClient struct{}

var _ profilev1connect.ProfileServiceClient = (*stubProfileServiceClient)(nil)

func (s *stubProfileServiceClient) GetById(_ context.Context, _ *connect.Request[profilepb.GetByIdRequest]) (*connect.Response[profilepb.GetByIdResponse], error) {
	return connect.NewResponse(&profilepb.GetByIdResponse{
		Data: &profilepb.ProfileObject{Id: "test_profile-id"},
	}), nil
}

func (s *stubProfileServiceClient) GetByContact(_ context.Context, _ *connect.Request[profilepb.GetByContactRequest]) (*connect.Response[profilepb.GetByContactResponse], error) {
	return connect.NewResponse(&profilepb.GetByContactResponse{
		Data: &profilepb.ProfileObject{Id: "test_profile-id"},
	}), nil
}

func (s *stubProfileServiceClient) Search(context.Context, *connect.Request[profilepb.SearchRequest]) (*connect.ServerStreamForClient[profilepb.SearchResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) Merge(context.Context, *connect.Request[profilepb.MergeRequest]) (*connect.Response[profilepb.MergeResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) Create(context.Context, *connect.Request[profilepb.CreateRequest]) (*connect.Response[profilepb.CreateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) Update(context.Context, *connect.Request[profilepb.UpdateRequest]) (*connect.Response[profilepb.UpdateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) AddContact(context.Context, *connect.Request[profilepb.AddContactRequest]) (*connect.Response[profilepb.AddContactResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) CreateContact(context.Context, *connect.Request[profilepb.CreateContactRequest]) (*connect.Response[profilepb.CreateContactResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) CreateContactVerification(context.Context, *connect.Request[profilepb.CreateContactVerificationRequest]) (*connect.Response[profilepb.CreateContactVerificationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) CheckVerification(context.Context, *connect.Request[profilepb.CheckVerificationRequest]) (*connect.Response[profilepb.CheckVerificationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) RemoveContact(context.Context, *connect.Request[profilepb.RemoveContactRequest]) (*connect.Response[profilepb.RemoveContactResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) SearchRoster(context.Context, *connect.Request[profilepb.SearchRosterRequest]) (*connect.ServerStreamForClient[profilepb.SearchRosterResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) AddRoster(context.Context, *connect.Request[profilepb.AddRosterRequest]) (*connect.Response[profilepb.AddRosterResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) RemoveRoster(context.Context, *connect.Request[profilepb.RemoveRosterRequest]) (*connect.Response[profilepb.RemoveRosterResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) AddAddress(context.Context, *connect.Request[profilepb.AddAddressRequest]) (*connect.Response[profilepb.AddAddressResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) AddRelationship(context.Context, *connect.Request[profilepb.AddRelationshipRequest]) (*connect.Response[profilepb.AddRelationshipResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) DeleteRelationship(context.Context, *connect.Request[profilepb.DeleteRelationshipRequest]) (*connect.Response[profilepb.DeleteRelationshipResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubProfileServiceClient) ListRelationship(context.Context, *connect.Request[profilepb.ListRelationshipRequest]) (*connect.ServerStreamForClient[profilepb.ListRelationshipResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// --- stub TenancyServiceClient ---

type stubTenancyServiceClient struct{}

var _ tenancyv1connect.TenancyServiceClient = (*stubTenancyServiceClient)(nil)

func (s *stubTenancyServiceClient) GetAccess(_ context.Context, _ *connect.Request[tenancypb.GetAccessRequest]) (*connect.Response[tenancypb.GetAccessResponse], error) {
	return connect.NewResponse(&tenancypb.GetAccessResponse{
		Data: &tenancypb.AccessObject{
			Id: "test_access-id",
			Partition: &tenancypb.PartitionObject{
				Id:       "test_partition-id",
				TenantId: "test_tenant-id",
			},
		},
	}), nil
}

func (s *stubTenancyServiceClient) GetTenant(context.Context, *connect.Request[tenancypb.GetTenantRequest]) (*connect.Response[tenancypb.GetTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) ListTenant(context.Context, *connect.Request[tenancypb.ListTenantRequest]) (*connect.ServerStreamForClient[tenancypb.ListTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) CreateTenant(context.Context, *connect.Request[tenancypb.CreateTenantRequest]) (*connect.Response[tenancypb.CreateTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) UpdateTenant(context.Context, *connect.Request[tenancypb.UpdateTenantRequest]) (*connect.Response[tenancypb.UpdateTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) RemoveTenant(context.Context, *connect.Request[tenancypb.RemoveTenantRequest]) (*connect.Response[tenancypb.RemoveTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) ListPartition(context.Context, *connect.Request[tenancypb.ListPartitionRequest]) (*connect.ServerStreamForClient[tenancypb.ListPartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) CreatePartition(context.Context, *connect.Request[tenancypb.CreatePartitionRequest]) (*connect.Response[tenancypb.CreatePartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) GetPartition(context.Context, *connect.Request[tenancypb.GetPartitionRequest]) (*connect.Response[tenancypb.GetPartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) GetPartitionParents(context.Context, *connect.Request[tenancypb.GetPartitionParentsRequest]) (*connect.Response[tenancypb.GetPartitionParentsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) RemovePartition(context.Context, *connect.Request[tenancypb.RemovePartitionRequest]) (*connect.Response[tenancypb.RemovePartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) UpdatePartition(context.Context, *connect.Request[tenancypb.UpdatePartitionRequest]) (*connect.Response[tenancypb.UpdatePartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) CreatePartitionRole(context.Context, *connect.Request[tenancypb.CreatePartitionRoleRequest]) (*connect.Response[tenancypb.CreatePartitionRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) ListPartitionRole(context.Context, *connect.Request[tenancypb.ListPartitionRoleRequest]) (*connect.ServerStreamForClient[tenancypb.ListPartitionRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) UpdatePartitionRole(context.Context, *connect.Request[tenancypb.UpdatePartitionRoleRequest]) (*connect.Response[tenancypb.UpdatePartitionRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) RemovePartitionRole(context.Context, *connect.Request[tenancypb.RemovePartitionRoleRequest]) (*connect.Response[tenancypb.RemovePartitionRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) CreatePage(context.Context, *connect.Request[tenancypb.CreatePageRequest]) (*connect.Response[tenancypb.CreatePageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) ListPage(context.Context, *connect.Request[tenancypb.ListPageRequest]) (*connect.ServerStreamForClient[tenancypb.ListPageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) GetPage(context.Context, *connect.Request[tenancypb.GetPageRequest]) (*connect.Response[tenancypb.GetPageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) UpdatePage(context.Context, *connect.Request[tenancypb.UpdatePageRequest]) (*connect.Response[tenancypb.UpdatePageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) RemovePage(context.Context, *connect.Request[tenancypb.RemovePageRequest]) (*connect.Response[tenancypb.RemovePageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) CreateAccess(context.Context, *connect.Request[tenancypb.CreateAccessRequest]) (*connect.Response[tenancypb.CreateAccessResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) ListAccess(context.Context, *connect.Request[tenancypb.ListAccessRequest]) (*connect.ServerStreamForClient[tenancypb.ListAccessResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) RemoveAccess(context.Context, *connect.Request[tenancypb.RemoveAccessRequest]) (*connect.Response[tenancypb.RemoveAccessResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) CreateAccessRole(context.Context, *connect.Request[tenancypb.CreateAccessRoleRequest]) (*connect.Response[tenancypb.CreateAccessRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) ListAccessRole(context.Context, *connect.Request[tenancypb.ListAccessRoleRequest]) (*connect.ServerStreamForClient[tenancypb.ListAccessRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) RemoveAccessRole(context.Context, *connect.Request[tenancypb.RemoveAccessRoleRequest]) (*connect.Response[tenancypb.RemoveAccessRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) CreateServiceAccount(context.Context, *connect.Request[tenancypb.CreateServiceAccountRequest]) (*connect.Response[tenancypb.CreateServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) GetServiceAccount(context.Context, *connect.Request[tenancypb.GetServiceAccountRequest]) (*connect.Response[tenancypb.GetServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) UpdateServiceAccount(context.Context, *connect.Request[tenancypb.UpdateServiceAccountRequest]) (*connect.Response[tenancypb.UpdateServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) ListServiceAccount(context.Context, *connect.Request[tenancypb.ListServiceAccountRequest]) (*connect.ServerStreamForClient[tenancypb.ListServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) RemoveServiceAccount(context.Context, *connect.Request[tenancypb.RemoveServiceAccountRequest]) (*connect.Response[tenancypb.RemoveServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) CreateClient(context.Context, *connect.Request[tenancypb.CreateClientRequest]) (*connect.Response[tenancypb.CreateClientResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) GetClient(context.Context, *connect.Request[tenancypb.GetClientRequest]) (*connect.Response[tenancypb.GetClientResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) ListClient(context.Context, *connect.Request[tenancypb.ListClientRequest]) (*connect.ServerStreamForClient[tenancypb.ListClientResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) UpdateClient(context.Context, *connect.Request[tenancypb.UpdateClientRequest]) (*connect.Response[tenancypb.UpdateClientResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubTenancyServiceClient) RemoveClient(context.Context, *connect.Request[tenancypb.RemoveClientRequest]) (*connect.Response[tenancypb.RemoveClientResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// --- BaseTestSuite ---

type BaseTestSuite struct {
	frametests.FrameBaseTestSuite
}

func initResources(_ context.Context) []definition.TestResource {
	pg := testpostgres.NewWithOpts("service_file", definition.WithUserName("ant"), definition.WithCredential("s3cr3t"))
	resources := []definition.TestResource{pg}
	return resources
}

func (bs *BaseTestSuite) SetupSuite() {
	bs.InitResourceFunc = initResources
	bs.FrameBaseTestSuite.SetupSuite()
}

func (bs *BaseTestSuite) GetProfileCli(_ context.Context) profilev1connect.ProfileServiceClient {
	return &stubProfileServiceClient{}
}

func (bs *BaseTestSuite) GetTenancyCli(_ context.Context) tenancyv1connect.TenancyServiceClient {
	return &stubTenancyServiceClient{}
}

func (bs *BaseTestSuite) TearDownSuite() {
	bs.FrameBaseTestSuite.TearDownSuite()
}

// WithTestDependancies Creates subtests with each known DependancyOption.
func (bs *BaseTestSuite) WithTestDependancies(t *testing.T, testFn func(t *testing.T, dep *definition.DependencyOption)) {
	options := []*definition.DependencyOption{
		definition.NewDependancyOption("default", util.RandomAlphaNumericString(DefaultRandomStringLength), bs.Resources()),
	}

	frametests.WithTestDependencies(t, options, testFn)
}
