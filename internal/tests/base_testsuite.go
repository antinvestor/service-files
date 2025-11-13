package tests

import (
	"context"
	"testing"

	"github.com/antinvestor/apis/go/common"
	partitionv1 "github.com/antinvestor/apis/go/partition/v1"
	partitionv1_mocks "github.com/antinvestor/apis/go/partition/v1_mocks"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	profilev1_mocks "github.com/antinvestor/apis/go/profile/v1_mocks"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/frametests/deps/testpostgres"
	"github.com/pitabwire/util"
	"go.uber.org/mock/gomock"
)

const (
	DefaultRandomStringLength = 8
)

type BaseTestSuite struct {
	frametests.FrameBaseTestSuite
}

func initResources(_ context.Context) []definition.TestResource {
	pg := testpostgres.NewWithOpts("service_files", definition.WithUserName("ant"), definition.WithPassword("s3cr3t"))
	resources := []definition.TestResource{pg}
	return resources
}

func (bs *BaseTestSuite) SetupSuite() {
	bs.InitResourceFunc = initResources
	bs.FrameBaseTestSuite.SetupSuite()
}

func (bs *BaseTestSuite) GetProfileCli(_ context.Context) *profilev1.ProfileClient {

	t := bs.T()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockProfileService := profilev1_mocks.NewMockProfileServiceClient(ctrl)
	mockProfileService.EXPECT().
		GetById(gomock.Any(), gomock.Any()).
		Return(&profilev1.GetByIdResponse{
			Data: &profilev1.ProfileObject{
				Id: "test_profile-id",
			},
		}, nil).AnyTimes()
	mockProfileService.EXPECT().
		GetByContact(gomock.Any(), gomock.Any()).
		Return(&profilev1.GetByContactResponse{
			Data: &profilev1.ProfileObject{
				Id: "test_profile-id",
			},
		}, nil).AnyTimes()

	profileCli := profilev1.Init(&common.GrpcClientBase{}, mockProfileService)
	return profileCli
}

func (bs *BaseTestSuite) GetPartitionCli(_ context.Context) *partitionv1.PartitionClient {

	t := bs.T()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPartitionService := partitionv1_mocks.NewMockPartitionServiceClient(ctrl)

	mockPartitionService.EXPECT().
		GetAccess(gomock.Any(), gomock.Any()).
		Return(&partitionv1.GetAccessResponse{Data: &partitionv1.AccessObject{
			AccessId: "test_access-id",
			Partition: &partitionv1.PartitionObject{
				Id:       "test_partition-id",
				TenantId: "test_tenant-id",
			},
		}}, nil).AnyTimes()

	profileCli := partitionv1.Init(&common.GrpcClientBase{}, mockPartitionService)
	return profileCli
}

func (bs *BaseTestSuite) TearDownSuite() {
	bs.FrameBaseTestSuite.TearDownSuite()
}

// WithTestDependancies Creates subtests with each known DependancyOption.
func (bs *BaseTestSuite) WithTestDependancies(t *testing.T, testFn func(t *testing.T, dep *definition.DependancyOption)) {
	options := []*definition.DependancyOption{
		definition.NewDependancyOption("default", util.RandomString(DefaultRandomStringLength), bs.Resources()),
	}

	frametests.WithTestDependancies(t, options, testFn)
}
