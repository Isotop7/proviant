package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
)

func TestGetHouseholdApplicationsHandler(t *testing.T) {
	t.Run("admin gets pending applications", func(t *testing.T) {
		env := setupHandlerTest(t)
		applicant := env.addMember(t, "applicant")
		env.DB.Create(&dbModel.HouseholdApplication{
			ApplicantID: applicant.ID,
			HouseholdID: env.Household.ID,
			Status:      dbModel.ApplicationStatusPending,
		})
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/household/applications", nil)

		GetHouseholdApplications(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var apps []dbModel.HouseholdApplication
		if err := json.Unmarshal(env.W.Body.Bytes(), &apps); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(apps) != 1 || apps[0].ApplicantID != applicant.ID {
			t.Errorf("applications = %+v, want one for applicant %d", apps, applicant.ID)
		}
	})

	t.Run("non-admin gets forbidden", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.useMember(member)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/household/applications", nil)

		GetHouseholdApplications(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})
}

func TestGetHouseholdSettingsHandler(t *testing.T) {
	t.Run("returns settings", func(t *testing.T) {
		env := setupHandlerTest(t)
		goalCount := 5
		env.Household.MonthlyWasteGoalType = "count"
		env.Household.MonthlyWasteGoalCount = &goalCount
		env.DB.Save(env.Household)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/household/settings", nil)

		GetHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.HouseholdSettingsResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.MonthlyWasteGoalType != "count" || resp.MonthlyWasteGoalCount == nil || *resp.MonthlyWasteGoalCount != 5 {
			t.Errorf("response = %+v, want count goal 5", resp)
		}
	})

	t.Run("unknown user returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.AppCtx.UserID = 9999
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/household/settings", nil)

		GetHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestUpdateHouseholdSettingsHandler(t *testing.T) {
	t.Run("valid count goal", func(t *testing.T) {
		env := setupHandlerTest(t)
		goalCount := 10
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateHouseholdSettingsRequest{
			MonthlyWasteGoalType:  "count",
			MonthlyWasteGoalCount: &goalCount,
		})

		UpdateHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Household
		if err := env.DB.First(&stored, env.Household.ID).Error; err != nil {
			t.Fatalf("load household: %v", err)
		}
		if stored.MonthlyWasteGoalType != "count" || stored.MonthlyWasteGoalCount == nil || *stored.MonthlyWasteGoalCount != 10 {
			t.Errorf("stored household = %+v, want count goal 10", stored)
		}
	})

	t.Run("valid percent goal", func(t *testing.T) {
		env := setupHandlerTest(t)
		goalPercent := 25.0
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateHouseholdSettingsRequest{
			MonthlyWasteGoalType:    "percent",
			MonthlyWasteGoalPercent: &goalPercent,
		})

		UpdateHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
	})

	t.Run("empty type clears goals", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateHouseholdSettingsRequest{})

		UpdateHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
	})

	t.Run("invalid goal type returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateHouseholdSettingsRequest{MonthlyWasteGoalType: "weekly"})

		UpdateHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("count type without count returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateHouseholdSettingsRequest{MonthlyWasteGoalType: "count"})

		UpdateHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("count type with negative count returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		negative := -1
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateHouseholdSettingsRequest{
			MonthlyWasteGoalType:  "count",
			MonthlyWasteGoalCount: &negative,
		})

		UpdateHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("percent type above 100 returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		tooHigh := 101.0
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateHouseholdSettingsRequest{
			MonthlyWasteGoalType:    "percent",
			MonthlyWasteGoalPercent: &tooHigh,
		})

		UpdateHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("malformed body returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/household/settings", bytesReaderString("{invalid"))
		env.Ctx.Request.Header.Set("Content-Type", "application/json")

		UpdateHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("non-admin gets forbidden", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.useMember(member)
		goalCount := 10
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateHouseholdSettingsRequest{
			MonthlyWasteGoalType:  "count",
			MonthlyWasteGoalCount: &goalCount,
		})

		UpdateHouseholdSettings(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})
}

func TestGetHouseholdActivityHandler(t *testing.T) {
	t.Run("returns activity entries", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.DB.Create(&dbModel.ActivityLog{
			HouseholdID: env.Household.ID,
			UserID:      &env.User.ID,
			UserName:    env.User.Username,
			Action:      dbModel.ActivityActionAdd,
			ProductID:   1,
			ProductName: "Milk",
			Quantity:    2,
			Timestamp:   time.Now(),
		})
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/household/activity?limit=10&offset=0", nil)

		GetHouseholdActivity(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.ActivityLogResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Total != 1 || len(resp.Activities) != 1 {
			t.Fatalf("response = %+v, want total 1 with one entry", resp)
		}
		if resp.Activities[0].ProductName != "Milk" || resp.Activities[0].Action != dbModel.ActivityActionAdd {
			t.Errorf("entry = %+v, want add/Milk", resp.Activities[0])
		}
		if resp.Limit != 10 || resp.Offset != 0 {
			t.Errorf("limit/offset = %d/%d, want 10/0", resp.Limit, resp.Offset)
		}
	})

	t.Run("limit is capped at 100", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/household/activity?limit=5000", nil)

		GetHouseholdActivity(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
		var resp apiModel.ActivityLogResponse
		_ = json.Unmarshal(env.W.Body.Bytes(), &resp)
		if resp.Limit != 100 {
			t.Errorf("limit = %d, want 100", resp.Limit)
		}
	})

	t.Run("unknown user returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.AppCtx.UserID = 9999
		env.Ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/household/activity", nil)

		GetHouseholdActivity(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("user lookup failure returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/household/activity", nil)
		appCtx := SetupTestAppContext(ctx, 1)

		GetHouseholdActivity(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}

func TestUpdateHouseholdMemberRoleHandler(t *testing.T) {
	t.Run("admin updates member role", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: itoa(member.ID)}}
		testutil.CreateTestRequest(env.Ctx, updateHouseholdMemberRoleRequest{Role: authentication.RoleViewer})

		UpdateHouseholdMemberRole(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored authentication.User
		if err := env.DB.First(&stored, member.ID).Error; err != nil {
			t.Fatalf("load member: %v", err)
		}
		if stored.Role != authentication.RoleViewer {
			t.Errorf("role = %q, want viewer", stored.Role)
		}
	})

	t.Run("invalid role returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: itoa(member.ID)}}
		testutil.CreateTestRequest(env.Ctx, updateHouseholdMemberRoleRequest{Role: "superuser"})

		UpdateHouseholdMemberRole(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("invalid user id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: "abc"}}
		testutil.CreateTestRequest(env.Ctx, updateHouseholdMemberRoleRequest{Role: authentication.RoleViewer})

		UpdateHouseholdMemberRole(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("non-admin gets forbidden", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		other := env.addMember(t, "other")
		env.useMember(member)
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: itoa(other.ID)}}
		testutil.CreateTestRequest(env.Ctx, updateHouseholdMemberRoleRequest{Role: authentication.RoleViewer})

		UpdateHouseholdMemberRole(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})

	t.Run("member outside household returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		outsider := testutil.CreateTestUser(env.DB, 0)
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: itoa(outsider.ID)}}
		testutil.CreateTestRequest(env.Ctx, updateHouseholdMemberRoleRequest{Role: authentication.RoleViewer})

		UpdateHouseholdMemberRole(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("last admin cannot demote self", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: itoa(env.User.ID)}}
		testutil.CreateTestRequest(env.Ctx, updateHouseholdMemberRoleRequest{Role: authentication.RoleMember})

		UpdateHouseholdMemberRole(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestRemoveHouseholdMemberHandler(t *testing.T) {
	t.Run("admin removes member", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: itoa(member.ID)}}

		RemoveHouseholdMember(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored authentication.User
		if err := env.DB.First(&stored, member.ID).Error; err != nil {
			t.Fatalf("load member: %v", err)
		}
		if stored.HouseholdID == env.Household.ID {
			t.Errorf("member still in household %d", env.Household.ID)
		}
	})

	t.Run("removing self returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: itoa(env.User.ID)}}

		RemoveHouseholdMember(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("member outside household returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		outsider := testutil.CreateTestUser(env.DB, 0)
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: itoa(outsider.ID)}}

		RemoveHouseholdMember(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("non-admin gets forbidden", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		other := env.addMember(t, "other")
		env.useMember(member)
		env.Ctx.Params = []gin.Param{{Key: "userId", Value: itoa(other.ID)}}

		RemoveHouseholdMember(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})
}

func TestCancelHouseholdApplicationHandler(t *testing.T) {
	t.Run("applicant cancels own application", func(t *testing.T) {
		env := setupHandlerTest(t)
		applicant := env.addMember(t, "applicant")
		application := dbModel.HouseholdApplication{
			ApplicantID: applicant.ID,
			HouseholdID: env.Household.ID,
			Status:      dbModel.ApplicationStatusPending,
		}
		env.DB.Create(&application)
		env.useMember(applicant)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(application.ID)}}

		CancelHouseholdApplication(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Model(&dbModel.HouseholdApplication{}).Where("id = ?", application.ID).Count(&count)
		if count != 0 {
			t.Errorf("application still present, count = %d", count)
		}
	})

	t.Run("cancelling another user's application returns 403", func(t *testing.T) {
		env := setupHandlerTest(t)
		applicant := env.addMember(t, "applicant")
		application := dbModel.HouseholdApplication{
			ApplicantID: applicant.ID,
			HouseholdID: env.Household.ID,
			Status:      dbModel.ApplicationStatusPending,
		}
		env.DB.Create(&application)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(application.ID)}}

		CancelHouseholdApplication(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}

		CancelHouseholdApplication(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestApproveHouseholdApplicationSuccess(t *testing.T) {
	env := setupHandlerTest(t)
	// Applicant starts outside the household with a non-member role, so the
	// assertions below can only pass if the handler moves and demotes them in.
	applicant := testutil.CreateTestUser(env.DB, 0)
	applicant.Username = "applicant"
	applicant.Role = authentication.RoleViewer
	if err := env.DB.Save(applicant).Error; err != nil {
		t.Fatalf("save applicant: %v", err)
	}
	application := dbModel.HouseholdApplication{
		ApplicantID: applicant.ID,
		HouseholdID: env.Household.ID,
		Status:      dbModel.ApplicationStatusPending,
	}
	env.DB.Create(&application)
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(application.ID)}}

	ApproveHouseholdApplication(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored authentication.User
	if err := env.DB.First(&stored, applicant.ID).Error; err != nil {
		t.Fatalf("load applicant: %v", err)
	}
	if stored.HouseholdID != env.Household.ID || stored.Role != authentication.RoleMember {
		t.Errorf("applicant = household %d role %q, want household %d role member", stored.HouseholdID, stored.Role, env.Household.ID)
	}
	var storedApp dbModel.HouseholdApplication
	if err := env.DB.First(&storedApp, application.ID).Error; err != nil {
		t.Fatalf("load application: %v", err)
	}
	if storedApp.Status != dbModel.ApplicationStatusApproved {
		t.Errorf("application status = %q, want approved", storedApp.Status)
	}
}

func TestRejectHouseholdApplicationSuccess(t *testing.T) {
	env := setupHandlerTest(t)
	applicant := env.addMember(t, "applicant")
	application := dbModel.HouseholdApplication{
		ApplicantID: applicant.ID,
		HouseholdID: env.Household.ID,
		Status:      dbModel.ApplicationStatusPending,
	}
	env.DB.Create(&application)
	env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(application.ID)}}

	RejectHouseholdApplication(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var storedApp dbModel.HouseholdApplication
	if err := env.DB.First(&storedApp, application.ID).Error; err != nil {
		t.Fatalf("load application: %v", err)
	}
	if storedApp.Status != dbModel.ApplicationStatusRejected {
		t.Errorf("application status = %q, want rejected", storedApp.Status)
	}
}

func TestApplyForHouseholdHandler(t *testing.T) {
	t.Run("successful application", func(t *testing.T) {
		env := setupHandlerTest(t)
		applicant := env.addMember(t, "applicant")
		env.useMember(applicant)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(env.Household.ID)}}

		ApplyForHousehold(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
	})

	t.Run("duplicate application returns 409", func(t *testing.T) {
		env := setupHandlerTest(t)
		applicant := env.addMember(t, "applicant")
		env.useMember(applicant)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(env.Household.ID)}}

		ApplyForHousehold(env.Ctx, env.AppCtx)
		if env.W.Code != http.StatusOK {
			t.Fatalf("first call status = %d, want 200", env.W.Code)
		}

		ctx2, w2 := env.freshCtx(applicant.ID)
		ctx2.Params = []gin.Param{{Key: "id", Value: itoa(env.Household.ID)}}
		appCtx2 := SetupTestAppContext(ctx2, applicant.ID)
		ApplyForHousehold(ctx2, appCtx2)

		if w2.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409", w2.Code)
		}
	})

	t.Run("missing household returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}

		ApplyForHousehold(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}

		ApplyForHousehold(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestUpdateHouseholdNameHandler(t *testing.T) {
	t.Run("admin renames household", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, updateHouseholdNameRequest{Name: "New Name"})

		UpdateHouseholdName(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Household
		if err := env.DB.First(&stored, env.Household.ID).Error; err != nil {
			t.Fatalf("load household: %v", err)
		}
		if stored.Name != "New Name" {
			t.Errorf("name = %q, want %q", stored.Name, "New Name")
		}
	})

	t.Run("empty name returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, updateHouseholdNameRequest{Name: ""})

		UpdateHouseholdName(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("non-admin gets forbidden", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.useMember(member)
		testutil.CreateTestRequest(env.Ctx, updateHouseholdNameRequest{Name: "New Name"})

		UpdateHouseholdName(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})
}

func TestCreateHouseholdHandler(t *testing.T) {
	t.Run("creates and switches household", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, createHouseholdRequest{Name: "My New Household"})

		CreateHousehold(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored authentication.User
		if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
			t.Fatalf("load user: %v", err)
		}
		if stored.HouseholdID == env.Household.ID {
			t.Errorf("user still in old household %d", env.Household.ID)
		}
	})

	t.Run("missing name returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, createHouseholdRequest{Name: ""})

		CreateHousehold(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestLeaveHouseholdHandler(t *testing.T) {
	env := setupHandlerTest(t)
	env.Ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/household/leave", nil)

	LeaveHousehold(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var stored authentication.User
	if err := env.DB.First(&stored, env.User.ID).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if stored.HouseholdID == env.Household.ID {
		t.Errorf("user still in old household %d", env.Household.ID)
	}
}
