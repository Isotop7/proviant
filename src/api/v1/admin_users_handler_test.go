package v1

import (
	"encoding/json"
	"net/http"
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

func TestGetHouseholdUsers(t *testing.T) {
	t.Run("returns household members", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.addMember(t, "member")

		GetHouseholdUsers(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var users []authentication.User
		if err := json.Unmarshal(env.W.Body.Bytes(), &users); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(users) != 2 {
			t.Errorf("users = %d, want 2", len(users))
		}
	})

	t.Run("unknown caller returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.AppCtx.UserID = 9999

		GetHouseholdUsers(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestUpdateHouseholdUser(t *testing.T) {
	t.Run("admin updates member", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(member.ID)}}
		testutil.CreateTestRequest(env.Ctx, updateAdminUserRequest{Username: "renamed", MailAddress: "renamed@example.com"})

		UpdateHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored authentication.User
		if err := env.DB.First(&stored, member.ID).Error; err != nil {
			t.Fatalf("load member: %v", err)
		}
		if stored.Username != "renamed" || stored.MailAddress != "renamed@example.com" {
			t.Errorf("member = %+v, want renamed/renamed@example.com", stored)
		}
	})

	t.Run("empty fields keep existing values", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(member.ID)}}
		testutil.CreateTestRequest(env.Ctx, updateAdminUserRequest{})

		UpdateHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored authentication.User
		if err := env.DB.First(&stored, member.ID).Error; err != nil {
			t.Fatalf("load member: %v", err)
		}
		if stored.Username != "member" || stored.MailAddress != "test@example.com" {
			t.Errorf("member = %+v, want unchanged values", stored)
		}
	})

	t.Run("non-admin caller returns 403", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		other := env.addMember(t, "other")
		env.useMember(member)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(other.ID)}}
		testutil.CreateTestRequest(env.Ctx, updateAdminUserRequest{Username: "renamed"})

		UpdateHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})

	t.Run("target outside household returns 403", func(t *testing.T) {
		env := setupHandlerTest(t)
		outsider := testutil.CreateTestUser(env.DB, 0)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(outsider.ID)}}
		testutil.CreateTestRequest(env.Ctx, updateAdminUserRequest{Username: "renamed"})

		UpdateHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})

	t.Run("missing target returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}
		testutil.CreateTestRequest(env.Ctx, updateAdminUserRequest{Username: "renamed"})

		UpdateHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}
		testutil.CreateTestRequest(env.Ctx, updateAdminUserRequest{Username: "renamed"})

		UpdateHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("malformed body returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(member.ID)}}
		env.Ctx.Request = newRawJSONRequest("{invalid")

		UpdateHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestDeleteHouseholdUser(t *testing.T) {
	t.Run("admin deletes member", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(member.ID)}}

		DeleteHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Model(&authentication.User{}).Where("id = ?", member.ID).Count(&count)
		if count != 0 {
			t.Errorf("user still present, count = %d", count)
		}
	})

	t.Run("deleting self returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(env.User.ID)}}

		DeleteHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("non-admin caller returns 403", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		other := env.addMember(t, "other")
		env.useMember(member)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(other.ID)}}

		DeleteHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})

	t.Run("target outside household returns 403", func(t *testing.T) {
		env := setupHandlerTest(t)
		outsider := testutil.CreateTestUser(env.DB, 0)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(outsider.ID)}}

		DeleteHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})

	t.Run("missing target returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}

		DeleteHouseholdUser(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})
}

func TestAdminResetUserPassword(t *testing.T) {
	t.Run("creates reset verification for member", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(member.ID)}}

		AdminResetUserPassword(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Model(&dbModel.EmailVerification{}).Where("user_id = ?", member.ID).Count(&count)
		if count != 1 {
			t.Errorf("email verifications = %d, want 1", count)
		}
	})

	t.Run("non-admin caller returns 403", func(t *testing.T) {
		env := setupHandlerTest(t)
		member := env.addMember(t, "member")
		other := env.addMember(t, "other")
		env.useMember(member)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(other.ID)}}

		AdminResetUserPassword(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})

	t.Run("missing target returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}

		AdminResetUserPassword(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}

		AdminResetUserPassword(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}
