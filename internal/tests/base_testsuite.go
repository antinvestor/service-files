package tests

import (
	"context"
	"testing"

	"github.com/antinvestor/apis/go/common"
	partitionv1 "github.com/antinvestor/apis/go/partition/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/pitabwire/frame/tests"
	"github.com/pitabwire/frame/tests/deps/testpostgres"
	"github.com/pitabwire/frame/tests/testdef"
	"github.com/pitabwire/util"
	"go.uber.org/mock/gomock"
)

const PostgresqlDBImage = "paradedb/paradedb:latest"

const (
	DefaultRandomStringLength = 8
)

type BaseTestSuite struct {
	tests.FrameBaseTestSuite
}

func initResources(_ context.Context) []testdef.TestResource {
	pg := testpostgres.NewPGDepWithCred(PostgresqlDBImage, "ant", "s3cr3t", "service_profile")
	resources := []testdef.TestResource{pg}
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
	mockProfileService := profilev1.NewMockProfileServiceClient(ctrl)
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
	mockPartitionService := partitionv1.NewMockPartitionServiceClient(ctrl)

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
func (bs *BaseTestSuite) WithTestDependancies(t *testing.T, testFn func(t *testing.T, dep *testdef.DependancyOption)) {
	options := []*testdef.DependancyOption{
		testdef.NewDependancyOption("default", util.RandomString(DefaultRandomStringLength), bs.Resources()),
	}

	tests.WithTestDependancies(t, options, testFn)
}
