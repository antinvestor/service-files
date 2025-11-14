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

func initResources(_ context.Context) []definition.TestResource {
	pg := testpostgres.NewWithOpts("service_files", definition.WithUserName("ant"), definition.WithPassword("s3cr3t"))
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

	mockPartitionService := partitionv1_mocks.NewPartitionServiceClientMock(bs.mc)

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
		definition.NewDependancyOption("default", util.RandomString(DefaultRandomStringLength), bs.Resources()),
	}

	frametests.WithTestDependencies(t, options, testFn)
}
