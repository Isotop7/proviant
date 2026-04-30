package onboarding

import (
	"net/http"
	"testing"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestGetOnboardingState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("no state returns completed", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.OnboardingState = dbModel.OnboardingState{}
		m.Users.Err = gorm.ErrRecordNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		GetOnboardingState(ctx)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing repos in ctx returns 500", func(t *testing.T) {
		ctx, w := repomocks.SetupGinContextWithMocks(repomocks.NewMockRepositoryContainer())
		ctx.Set("repos", nil)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		GetOnboardingState(ctx)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestUpdateOnboardingProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("userID 0 returns 401", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 0, testutil.TokenIdentityKey)
		testutil.CreateTestRequest(ctx, map[string]string{"displayName": "test"})
		UpdateOnboardingProfile(ctx)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		testutil.CreateTestRequest(ctx, map[string]string{"displayName": "Test User"})
		UpdateOnboardingProfile(ctx)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestCreateOnboardingHousehold(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing name returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		testutil.CreateTestRequest(ctx, map[string]string{"name": ""})
		CreateOnboardingHousehold(ctx)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		testutil.CreateTestRequest(ctx, map[string]string{"name": "Test Household"})
		CreateOnboardingHousehold(ctx)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestJoinOnboardingByInvite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid token returns 404", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Invitations.Err = errors.ErrInvitationNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		testutil.CreateTestRequest(ctx, map[string]string{"token": "invalid"})
		JoinOnboardingByInvite(ctx)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGetAvailableHouseholds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user lookup err returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.Err = gorm.ErrInvalidData
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		GetAvailableHouseholds(ctx)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("empty list returns empty array", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Households = []dbModel.HouseholdWithMemberCount{}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		GetAvailableHouseholds(ctx)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns households", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Households = []dbModel.HouseholdWithMemberCount{
			{Household: dbModel.Household{Model: gorm.Model{ID: 1}, Name: "Household 1"}, MemberCount: 2},
		}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		GetAvailableHouseholds(ctx)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestApplyForHousehold(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("householdID 0 returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		testutil.CreateTestRequest(ctx, map[string]uint{"householdId": 0})
		ApplyForHousehold(ctx)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Err = errors.ErrHouseholdNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		testutil.CreateTestRequest(ctx, map[string]uint{"householdId": 999})
		ApplyForHousehold(ctx)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("already pending returns 409", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Households.Err = errors.ErrApplicationAlreadyPending
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		testutil.CreateTestRequest(ctx, map[string]uint{"householdId": 1})
		ApplyForHousehold(ctx)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		testutil.CreateTestRequest(ctx, map[string]uint{"householdId": 1})
		ApplyForHousehold(ctx)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestCompleteOnboarding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		CompleteOnboarding(ctx)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
