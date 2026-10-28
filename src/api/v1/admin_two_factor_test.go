package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"
)

// setupAdminTwoFactorTest wires an admin and a member who has a second factor.
func setupAdminTwoFactorTest(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *authentication.User, *authentication.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)

	admin := testutil.CreateTestUser(db, 0)
	adminHousehold := testutil.CreateTestHousehold(db, admin.ID)
	admin.HouseholdID = adminHousehold.ID
	admin.Role = authentication.RoleAdmin
	if err := db.Save(admin).Error; err != nil {
		t.Fatalf("save admin: %v", err)
	}

	member := testutil.CreateTestUser(db, adminHousehold.ID)
	member.HouseholdID = adminHousehold.ID
	member.Role = authentication.RoleMember
	if err := db.Model(member).Updates(map[string]any{
		"two_factor_enabled": true,
		"two_factor_secret":  "encrypted",
	}).Error; err != nil {
		t.Fatalf("enable the second factor of the member: %v", err)
	}

	ctx, w := testutil.SetupGinContext(db)
	testutil.MockJWTClaims(ctx, admin.ID)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	ctx.Set(util.ContextKeyProviantConfig, twoFactorHandlerConfig())
	// A typed nil skips the mail; the reset itself must work without SMTP.
	ctx.Set(util.ContextKeyNotificationController, (*controllers.NotificationController)(nil))

	return ctx, w, admin, member
}

// The admin-triggered password reset has to land in the password reset store.
// The email verification store only marks the address as verified, so a member
// who followed that link never changed their password.
func TestAdminResetUserPasswordCreatesAPasswordReset(t *testing.T) {
	ctx, w, admin, member := setupAdminTwoFactorTest(t)

	ctx.Params = gin.Params{{Key: "id", Value: uintToString(member.ID)}}
	AdminResetUserPassword(ctx, SetupTestAppContext(ctx, admin.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("AdminResetUserPassword() = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	db := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)

	var reset dbModel.PasswordReset
	if err := db.Where("user_id = ?", member.ID).First(&reset).Error; err != nil {
		t.Fatalf("no password reset token was created for the member: %v", err)
	}

	var verification dbModel.EmailVerification
	err := db.Where("user_id = ?", member.ID).First(&verification).Error
	if err == nil {
		t.Error("the reset created an email verification token, which never changes the password")
	}
}

func TestAdminResetUserTwoFactor(t *testing.T) {
	ctx, w, admin, member := setupAdminTwoFactorTest(t)

	ctx.Params = gin.Params{{Key: "id", Value: uintToString(member.ID)}}
	AdminResetUserTwoFactor(ctx, SetupTestAppContext(ctx, admin.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("AdminResetUserTwoFactor() = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var stored authentication.User
	if err := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB).First(&stored, member.ID).Error; err != nil {
		t.Fatalf("reload the member: %v", err)
	}
	if stored.TwoFactorEnabled {
		t.Error("the second factor was not cleared")
	}
	if stored.TwoFactorSecret != "" {
		t.Error("the secret was not cleared")
	}
}

// Clearing the factor alone does not evict somebody who knew the password —
// they sign in again and re-enrol. The reset therefore also triggers the
// password reset, which is the part that removes them.
//
// The token has to land in the password reset store: the email verification
// link only marks the address as verified and never changes the password.
func TestAdminResetUserTwoFactorTriggersThePasswordReset(t *testing.T) {
	ctx, w, admin, member := setupAdminTwoFactorTest(t)

	ctx.Params = gin.Params{{Key: "id", Value: uintToString(member.ID)}}
	AdminResetUserTwoFactor(ctx, SetupTestAppContext(ctx, admin.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("AdminResetUserTwoFactor() = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	db := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB)

	var reset dbModel.PasswordReset
	if err := db.Where("user_id = ?", member.ID).First(&reset).Error; err != nil {
		t.Fatalf("no password reset token was created for the member: %v", err)
	}

	var verification dbModel.EmailVerification
	err := db.Where("user_id = ?", member.ID).First(&verification).Error
	if err == nil {
		t.Error("the reset created an email verification token, which never changes the password")
	}
}

func TestAdminResetUserTwoFactorWithoutASecondFactor(t *testing.T) {
	ctx, w, admin, member := setupAdminTwoFactorTest(t)

	if err := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB).Model(&authentication.User{}).
		Where("id = ?", member.ID).
		Updates(map[string]any{"two_factor_enabled": false, "two_factor_secret": ""}).Error; err != nil {
		t.Fatalf("clear the second factor: %v", err)
	}

	ctx.Params = gin.Params{{Key: "id", Value: uintToString(member.ID)}}
	AdminResetUserTwoFactor(ctx, SetupTestAppContext(ctx, admin.ID))

	if w.Code != http.StatusBadRequest {
		t.Errorf("AdminResetUserTwoFactor() = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAdminResetUserTwoFactorRejectsAMemberOfAnotherHousehold(t *testing.T) {
	ctx, w, admin, _ := setupAdminTwoFactorTest(t)

	other := testutil.CreateTestUser(ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB), 0)
	otherHousehold := testutil.CreateTestHousehold(ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB), other.ID)
	other.HouseholdID = otherHousehold.ID
	if err := ctx.MustGet(util.ContextKeyDBHandle).(*gorm.DB).Save(other).Error; err != nil {
		t.Fatalf("save the other user: %v", err)
	}

	ctx.Params = gin.Params{{Key: "id", Value: uintToString(other.ID)}}
	AdminResetUserTwoFactor(ctx, SetupTestAppContext(ctx, admin.ID))

	if w.Code == http.StatusOK {
		t.Errorf("AdminResetUserTwoFactor() = %d, want a refusal", w.Code)
	}
}

// The admin view tells the panel which members have a second factor, so the
// admin can see what to clear.
func TestHouseholdUserResponseCarriesTheSecondFactorState(t *testing.T) {
	user := &authentication.User{Username: "alice", TwoFactorEnabled: true}

	response := apiModel.NewHouseholdUserResponse(user)

	if !response.TwoFactorEnabled {
		t.Error("twoFactorEnabled = false, want true")
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !containsJSONKey(data, "twoFactorEnabled") {
		t.Errorf("the response does not carry the field: %s", data)
	}
}

func containsJSONKey(data []byte, key string) bool {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return false
	}
	_, ok := decoded[key]
	return ok
}

func uintToString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
