package v1

import (
	"encoding/json"
	"net/http"
	"testing"

	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"

	"github.com/gin-gonic/gin"
)

func TestListStorageLocationsHandler(t *testing.T) {
	env := setupHandlerTest(t)
	testutil.CreateTestStorageLocation(env.DB, env.Household.ID)

	ListStorageLocations(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	var locations []dbModel.StorageLocation
	if err := json.Unmarshal(env.W.Body.Bytes(), &locations); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(locations) != 1 || locations[0].Name != "Fridge" {
		t.Errorf("locations = %+v, want one Fridge", locations)
	}
}

func TestCreateStorageLocationHandler(t *testing.T) {
	t.Run("creates with default icon", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, storageLocationRequest{Name: "Cellar", SortOrder: 3})

		CreateStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var loc dbModel.StorageLocation
		if err := json.Unmarshal(env.W.Body.Bytes(), &loc); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if loc.Name != "Cellar" || loc.Icon != "📦" || loc.HouseholdID != env.Household.ID {
			t.Errorf("location = %+v, want Cellar/📦 in household %d", loc, env.Household.ID)
		}
	})

	t.Run("creates with explicit icon", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, storageLocationRequest{Name: "Cellar", Icon: "🍷"})

		CreateStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", env.W.Code)
		}
		var loc dbModel.StorageLocation
		_ = json.Unmarshal(env.W.Body.Bytes(), &loc)
		if loc.Icon != "🍷" {
			t.Errorf("icon = %q, want 🍷", loc.Icon)
		}
	})

	t.Run("missing name returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, storageLocationRequest{Icon: "🍷"})

		CreateStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestUpdateStorageLocationHandler(t *testing.T) {
	t.Run("updates location", func(t *testing.T) {
		env := setupHandlerTest(t)
		location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(location.ID)}}
		testutil.CreateTestRequest(env.Ctx, storageLocationRequest{Name: "Garage", Icon: "🚗", SortOrder: 5})

		UpdateStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.StorageLocation
		if err := env.DB.First(&stored, location.ID).Error; err != nil {
			t.Fatalf("load location: %v", err)
		}
		if stored.Name != "Garage" || stored.Icon != "🚗" || stored.SortOrder != 5 {
			t.Errorf("location = %+v, want Garage/🚗/5", stored)
		}
	})

	t.Run("missing location returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}
		testutil.CreateTestRequest(env.Ctx, storageLocationRequest{Name: "Garage"})

		UpdateStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("location of another household returns 403", func(t *testing.T) {
		env := setupHandlerTest(t)
		otherUser := testutil.CreateTestUser(env.DB, 0)
		otherHousehold := testutil.CreateTestHousehold(env.DB, otherUser.ID)
		otherUser.HouseholdID = otherHousehold.ID
		env.DB.Save(otherUser)
		foreign := testutil.CreateTestStorageLocation(env.DB, otherHousehold.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(foreign.ID)}}
		testutil.CreateTestRequest(env.Ctx, storageLocationRequest{Name: "Garage"})

		UpdateStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}
		testutil.CreateTestRequest(env.Ctx, storageLocationRequest{Name: "Garage"})

		UpdateStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("missing name returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(location.ID)}}
		testutil.CreateTestRequest(env.Ctx, storageLocationRequest{})

		UpdateStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestDeleteStorageLocationHandler(t *testing.T) {
	t.Run("deletes location and unassigns products", func(t *testing.T) {
		env := setupHandlerTest(t)
		location := testutil.CreateTestStorageLocation(env.DB, env.Household.ID)
		product := testutil.CreateTestProduct(env.DB, env.Household.ID, env.User.ID)
		product.StorageLocationID = &location.ID
		env.DB.Save(product)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(location.ID)}}

		DeleteStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Model(&dbModel.StorageLocation{}).Where("id = ?", location.ID).Count(&count)
		if count != 0 {
			t.Errorf("location still present, count = %d", count)
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.StorageLocationID != nil {
			t.Errorf("product still assigned to location %v", *stored.StorageLocationID)
		}
	})

	t.Run("missing location returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}

		DeleteStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("location of another household returns 403", func(t *testing.T) {
		env := setupHandlerTest(t)
		otherUser := testutil.CreateTestUser(env.DB, 0)
		otherHousehold := testutil.CreateTestHousehold(env.DB, otherUser.ID)
		otherUser.HouseholdID = otherHousehold.ID
		env.DB.Save(otherUser)
		foreign := testutil.CreateTestStorageLocation(env.DB, otherHousehold.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(foreign.ID)}}

		DeleteStorageLocation(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", env.W.Code)
		}
	})
}
