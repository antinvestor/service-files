package connection_test

import (
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
)

type policyInput struct {
	id, name, description, owner string
	days                         int
	isDefault, isSystem          bool
}

func (p policyInput) GetID() string          { return p.id }
func (p policyInput) GetName() string        { return p.name }
func (p policyInput) GetDescription() string { return p.description }
func (p policyInput) GetRetentionDays() int  { return p.days }
func (p policyInput) GetIsDefault() bool     { return p.isDefault }
func (p policyInput) GetIsSystem() bool      { return p.isSystem }
func (p policyInput) GetOwnerID() string     { return p.owner }

type retentionInput struct {
	mediaID, policyID string
	expiresAt         *time.Time
	locked            bool
}

func (r retentionInput) GetMediaID() string       { return r.mediaID }
func (r retentionInput) GetPolicyID() string      { return r.policyID }
func (r retentionInput) GetExpiresAt() *time.Time { return r.expiresAt }
func (r retentionInput) GetIsLocked() bool        { return r.locked }

func (suite *ConnectionTestSuite) TestUpdatePolicy_PersistsOnTargetRow() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)
		db := &connection.Database{WorkManager: svc.WorkManager(), RetentionPolicyRepo: res.RetentionPolicyRepo}

		target := policyInput{id: util.IDString(), name: "short", description: "d", owner: "owner-1", days: 7}
		other := policyInput{id: util.IDString(), name: "other", description: "o", owner: "owner-1", days: 30}
		require.NoError(t, db.CreatePolicy(ctx, target))
		require.NoError(t, db.CreatePolicy(ctx, other))

		updated := target
		updated.name, updated.description, updated.days, updated.isDefault = "long", "updated", 90, true
		require.NoError(t, db.UpdatePolicy(ctx, updated))

		got, err := db.GetPolicy(ctx, target.id)
		require.NoError(t, err)
		require.Equal(t, "long", got.Name())
		require.Equal(t, "updated", got.Description())
		require.Equal(t, 90, got.RetentionDays())
		require.True(t, got.IsDefault())

		untouched, err := db.GetPolicy(ctx, other.id)
		require.NoError(t, err)
		require.Equal(t, "other", untouched.Name())
		require.Equal(t, 30, untouched.RetentionDays())

		require.Error(t, db.UpdatePolicy(ctx, policyInput{id: util.IDString(), name: "x", days: 1}),
			"updating a missing policy must report not found")
	})
}

func (suite *ConnectionTestSuite) TestLockRetention_TogglesTargetMedia() {
	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res := suite.CreateService(t, dep)
		db := &connection.Database{
			WorkManager:         svc.WorkManager(),
			RetentionPolicyRepo: res.RetentionPolicyRepo,
			FileRetentionRepo:   res.FileRetentionRepo,
		}

		policy := policyInput{id: util.IDString(), name: "p", owner: "owner-1", days: 7}
		require.NoError(t, db.CreatePolicy(ctx, policy))
		mediaID, otherMedia := util.IDString(), util.IDString()
		require.NoError(t, db.ApplyRetention(ctx, retentionInput{mediaID: mediaID, policyID: policy.id}))
		require.NoError(t, db.ApplyRetention(ctx, retentionInput{mediaID: otherMedia, policyID: policy.id}))

		for _, locked := range []bool{true, false} {
			require.NoError(t, db.LockRetention(ctx, mediaID, locked))
			got, err := db.GetRetention(ctx, mediaID)
			require.NoError(t, err)
			require.Equal(t, locked, got.IsLocked(), "lock state must persist")
		}

		require.NoError(t, db.LockRetention(ctx, mediaID, true))
		untouched, err := db.GetRetention(ctx, otherMedia)
		require.NoError(t, err)
		require.False(t, untouched.IsLocked(), "other media must not be locked")

		require.Error(t, db.LockRetention(ctx, util.IDString(), true),
			"locking media without a retention record must report not found")
	})
}
