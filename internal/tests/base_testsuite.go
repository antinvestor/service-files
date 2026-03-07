package tests

import (
	"context"
	"testing"

	"buf.build/gen/go/antinvestor/partition/connectrpc/go/partition/v1/partitionv1connect"
	partitionpb "buf.build/gen/go/antinvestor/partition/protocolbuffers/go/partition/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilepb "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"connectrpc.com/connect"
	partitionv1_mocks "github.com/antinvestor/apis/go/partition/mocks"
	profilev1_mocks "github.com/antinvestor/apis/go/profile/mocks"
	"github.com/gojuno/minimock/v3"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/frametests/deps/testpostgres"
	"github.com/pitabwire/util"
)

const (
	DefaultRandomStringLength = 8
)

type BaseTestSuite struct {
	frametests.FrameBaseTestSuite

	mc *minimock.Controller
}

// partitionServiceClientCompatMock bridges the gap between the latest
// generated Connect client and the older mock surface published in apis.
// Tests in this repo only exercise GetAccess, so the added methods can stay
// as no-op stubs until the upstream mocks are regenerated.
type partitionServiceClientCompatMock struct {
	*partitionv1_mocks.PartitionServiceClientMock
}

func (m *partitionServiceClientCompatMock) RemoveTenant(context.Context, *connect.Request[partitionpb.RemoveTenantRequest]) (*connect.Response[partitionpb.RemoveTenantResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) RemovePartition(context.Context, *connect.Request[partitionpb.RemovePartitionRequest]) (*connect.Response[partitionpb.RemovePartitionResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) UpdatePartitionRole(context.Context, *connect.Request[partitionpb.UpdatePartitionRoleRequest]) (*connect.Response[partitionpb.UpdatePartitionRoleResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) ListPage(context.Context, *connect.Request[partitionpb.ListPageRequest]) (*connect.ServerStreamForClient[partitionpb.ListPageResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) UpdatePage(context.Context, *connect.Request[partitionpb.UpdatePageRequest]) (*connect.Response[partitionpb.UpdatePageResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) ListAccess(context.Context, *connect.Request[partitionpb.ListAccessRequest]) (*connect.ServerStreamForClient[partitionpb.ListAccessResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) UpdateServiceAccount(context.Context, *connect.Request[partitionpb.UpdateServiceAccountRequest]) (*connect.Response[partitionpb.UpdateServiceAccountResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) CreateClient(context.Context, *connect.Request[partitionpb.CreateClientRequest]) (*connect.Response[partitionpb.CreateClientResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) GetClient(context.Context, *connect.Request[partitionpb.GetClientRequest]) (*connect.Response[partitionpb.GetClientResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) ListClient(context.Context, *connect.Request[partitionpb.ListClientRequest]) (*connect.ServerStreamForClient[partitionpb.ListClientResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) UpdateClient(context.Context, *connect.Request[partitionpb.UpdateClientRequest]) (*connect.Response[partitionpb.UpdateClientResponse], error) {
	return nil, nil
}

func (m *partitionServiceClientCompatMock) RemoveClient(context.Context, *connect.Request[partitionpb.RemoveClientRequest]) (*connect.Response[partitionpb.RemoveClientResponse], error) {
	return nil, nil
}

func initResources(_ context.Context) []definition.TestResource {
	pg := testpostgres.NewWithOpts("service_files", definition.WithUserName("ant"), definition.WithCredential("s3cr3t"))
	resources := []definition.TestResource{pg}
	return resources
}

func (bs *BaseTestSuite) SetupSuite() {

	bs.InitResourceFunc = initResources
	bs.mc = minimock.NewController(bs.T())
	bs.FrameBaseTestSuite.SetupSuite()
}

func (bs *BaseTestSuite) GetProfileCli(_ context.Context) profilev1connect.ProfileServiceClient {

	mockProfileService := profilev1_mocks.NewProfileServiceClientMock(bs.mc)
	mockProfileService.GetByIdMock.Return(&connect.Response[profilepb.GetByIdResponse]{
		Msg: &profilepb.GetByIdResponse{
			Data: &profilepb.ProfileObject{
				Id: "test_profile-id",
			},
		},
	}, nil)
	mockProfileService.GetByContactMock.Return(&connect.Response[profilepb.GetByContactResponse]{
		Msg: &profilepb.GetByContactResponse{
			Data: &profilepb.ProfileObject{
				Id: "test_profile-id",
			},
		},
	}, nil)

	return mockProfileService
}

func (bs *BaseTestSuite) GetPartitionCli(_ context.Context) partitionv1connect.PartitionServiceClient {

	mockPartitionService := &partitionServiceClientCompatMock{
		PartitionServiceClientMock: partitionv1_mocks.NewPartitionServiceClientMock(bs.mc),
	}

	mockPartitionService.GetAccessMock.Return(&connect.Response[partitionpb.GetAccessResponse]{
		Msg: &partitionpb.GetAccessResponse{Data: &partitionpb.AccessObject{
			Id: "test_access-id",
			Partition: &partitionpb.PartitionObject{
				Id:       "test_partition-id",
				TenantId: "test_tenant-id",
			},
		}},
	}, nil)

	return mockPartitionService
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
