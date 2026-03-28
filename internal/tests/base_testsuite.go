package tests

import (
	"context"
	"errors"
	"testing"

	"buf.build/gen/go/antinvestor/partition/connectrpc/go/partition/v1/partitionv1connect"
	partitionpb "buf.build/gen/go/antinvestor/partition/protocolbuffers/go/partition/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilepb "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
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

// --- stub PartitionServiceClient ---

type stubPartitionServiceClient struct{}

var _ partitionv1connect.PartitionServiceClient = (*stubPartitionServiceClient)(nil)

func (s *stubPartitionServiceClient) GetAccess(_ context.Context, _ *connect.Request[partitionpb.GetAccessRequest]) (*connect.Response[partitionpb.GetAccessResponse], error) {
	return connect.NewResponse(&partitionpb.GetAccessResponse{
		Data: &partitionpb.AccessObject{
			Id: "test_access-id",
			Partition: &partitionpb.PartitionObject{
				Id:       "test_partition-id",
				TenantId: "test_tenant-id",
			},
		},
	}), nil
}

func (s *stubPartitionServiceClient) GetTenant(context.Context, *connect.Request[partitionpb.GetTenantRequest]) (*connect.Response[partitionpb.GetTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) ListTenant(context.Context, *connect.Request[partitionpb.ListTenantRequest]) (*connect.ServerStreamForClient[partitionpb.ListTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) CreateTenant(context.Context, *connect.Request[partitionpb.CreateTenantRequest]) (*connect.Response[partitionpb.CreateTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) UpdateTenant(context.Context, *connect.Request[partitionpb.UpdateTenantRequest]) (*connect.Response[partitionpb.UpdateTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) RemoveTenant(context.Context, *connect.Request[partitionpb.RemoveTenantRequest]) (*connect.Response[partitionpb.RemoveTenantResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) ListPartition(context.Context, *connect.Request[partitionpb.ListPartitionRequest]) (*connect.ServerStreamForClient[partitionpb.ListPartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) CreatePartition(context.Context, *connect.Request[partitionpb.CreatePartitionRequest]) (*connect.Response[partitionpb.CreatePartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) GetPartition(context.Context, *connect.Request[partitionpb.GetPartitionRequest]) (*connect.Response[partitionpb.GetPartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) GetPartitionParents(context.Context, *connect.Request[partitionpb.GetPartitionParentsRequest]) (*connect.Response[partitionpb.GetPartitionParentsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) RemovePartition(context.Context, *connect.Request[partitionpb.RemovePartitionRequest]) (*connect.Response[partitionpb.RemovePartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) UpdatePartition(context.Context, *connect.Request[partitionpb.UpdatePartitionRequest]) (*connect.Response[partitionpb.UpdatePartitionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) CreatePartitionRole(context.Context, *connect.Request[partitionpb.CreatePartitionRoleRequest]) (*connect.Response[partitionpb.CreatePartitionRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) ListPartitionRole(context.Context, *connect.Request[partitionpb.ListPartitionRoleRequest]) (*connect.ServerStreamForClient[partitionpb.ListPartitionRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) UpdatePartitionRole(context.Context, *connect.Request[partitionpb.UpdatePartitionRoleRequest]) (*connect.Response[partitionpb.UpdatePartitionRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) RemovePartitionRole(context.Context, *connect.Request[partitionpb.RemovePartitionRoleRequest]) (*connect.Response[partitionpb.RemovePartitionRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) CreatePage(context.Context, *connect.Request[partitionpb.CreatePageRequest]) (*connect.Response[partitionpb.CreatePageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) ListPage(context.Context, *connect.Request[partitionpb.ListPageRequest]) (*connect.ServerStreamForClient[partitionpb.ListPageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) GetPage(context.Context, *connect.Request[partitionpb.GetPageRequest]) (*connect.Response[partitionpb.GetPageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) UpdatePage(context.Context, *connect.Request[partitionpb.UpdatePageRequest]) (*connect.Response[partitionpb.UpdatePageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) RemovePage(context.Context, *connect.Request[partitionpb.RemovePageRequest]) (*connect.Response[partitionpb.RemovePageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) CreateAccess(context.Context, *connect.Request[partitionpb.CreateAccessRequest]) (*connect.Response[partitionpb.CreateAccessResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) ListAccess(context.Context, *connect.Request[partitionpb.ListAccessRequest]) (*connect.ServerStreamForClient[partitionpb.ListAccessResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) RemoveAccess(context.Context, *connect.Request[partitionpb.RemoveAccessRequest]) (*connect.Response[partitionpb.RemoveAccessResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) CreateAccessRole(context.Context, *connect.Request[partitionpb.CreateAccessRoleRequest]) (*connect.Response[partitionpb.CreateAccessRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) ListAccessRole(context.Context, *connect.Request[partitionpb.ListAccessRoleRequest]) (*connect.ServerStreamForClient[partitionpb.ListAccessRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) RemoveAccessRole(context.Context, *connect.Request[partitionpb.RemoveAccessRoleRequest]) (*connect.Response[partitionpb.RemoveAccessRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) CreateServiceAccount(context.Context, *connect.Request[partitionpb.CreateServiceAccountRequest]) (*connect.Response[partitionpb.CreateServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) GetServiceAccount(context.Context, *connect.Request[partitionpb.GetServiceAccountRequest]) (*connect.Response[partitionpb.GetServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) UpdateServiceAccount(context.Context, *connect.Request[partitionpb.UpdateServiceAccountRequest]) (*connect.Response[partitionpb.UpdateServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) ListServiceAccount(context.Context, *connect.Request[partitionpb.ListServiceAccountRequest]) (*connect.ServerStreamForClient[partitionpb.ListServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) RemoveServiceAccount(context.Context, *connect.Request[partitionpb.RemoveServiceAccountRequest]) (*connect.Response[partitionpb.RemoveServiceAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) CreateClient(context.Context, *connect.Request[partitionpb.CreateClientRequest]) (*connect.Response[partitionpb.CreateClientResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) GetClient(context.Context, *connect.Request[partitionpb.GetClientRequest]) (*connect.Response[partitionpb.GetClientResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) ListClient(context.Context, *connect.Request[partitionpb.ListClientRequest]) (*connect.ServerStreamForClient[partitionpb.ListClientResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) UpdateClient(context.Context, *connect.Request[partitionpb.UpdateClientRequest]) (*connect.Response[partitionpb.UpdateClientResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *stubPartitionServiceClient) RemoveClient(context.Context, *connect.Request[partitionpb.RemoveClientRequest]) (*connect.Response[partitionpb.RemoveClientResponse], error) {
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

func (bs *BaseTestSuite) GetPartitionCli(_ context.Context) partitionv1connect.PartitionServiceClient {
	return &stubPartitionServiceClient{}
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
