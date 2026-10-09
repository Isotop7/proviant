package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// setupHouseholdRoleRouter mounts RequireHouseholdAdmin behind a stand-in for
// the JWT/UserContext middlewares, which only supply dbHandle and userID.
func setupHouseholdRoleRouter(t *testing.T, role string) (*gin.Engine, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	user.Role = role
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to set role: %v", err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(util.ContextKeyDBHandle, db)
		c.Set(util.ContextKeyUserID, user.ID)
		c.Next()
	})
	r.GET("/admin", RequireHouseholdAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"householdId": c.GetUint(util.ContextKeyHouseholdID)})
	})
	return r, household.ID
}

func TestRequireHouseholdAdmin(t *testing.T) {
	t.Run("admin passes and household is stamped", func(t *testing.T) {
		r, householdID := setupHouseholdRoleRouter(t, authentication.RoleAdmin)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/admin", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), fmt.Sprintf(`"householdId":%d`, householdID))
	})

	t.Run("member is rejected with 403", func(t *testing.T) {
		r, _ := setupHouseholdRoleRouter(t, authentication.RoleMember)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/admin", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
