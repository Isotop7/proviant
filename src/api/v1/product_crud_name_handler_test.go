package v1

import (
	"net/http"
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"

	"gorm.io/gorm"
)

// A household that names its staple knows better than the label Open Food
// Facts prints: the name the request carries is the name the row keeps.
func TestCreateProductKeepsTheClientsName(t *testing.T) {
	t.Run("a client name wins over the barcode lookup", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Set("offacntrl", &MockOpenFoodFactsAPIController{
			MockGetDataset: func(barcode string) (dbModel.Product, error) {
				return dbModel.Product{
					Barcode:     barcode,
					ProductName: "Vollmilch 3.5%",
					PackSize:    "1 l",
				}, nil
			},
		})
		env.Ctx.Request = newRawJSONRequest(`{"barcode":"4006381333931","productName":"  Milch  ","packSize":"  1   l  "}`)

		CreateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, "barcode = ?", "4006381333931").Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.ProductName != "Milch" {
			t.Errorf("name = %q, want Milch (the household's own naming)", stored.ProductName)
		}
		if stored.PackSize != "1 l" {
			t.Errorf("pack size = %q, want 1 l", stored.PackSize)
		}
	})

	t.Run("the barcode lookup names the product when the request names none", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Set("offacntrl", &MockOpenFoodFactsAPIController{
			MockGetDataset: func(barcode string) (dbModel.Product, error) {
				return dbModel.Product{
					Barcode:     barcode,
					ProductName: "Vollmilch 3.5%",
					PackSize:    "1 l",
				}, nil
			},
		})
		env.Ctx.Request = newRawJSONRequest(`{"barcode":"4006381333931"}`)

		CreateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, "barcode = ?", "4006381333931").Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.ProductName != "Vollmilch 3.5%" {
			t.Errorf("name = %q, want the name the barcode lookup answered", stored.ProductName)
		}
		if stored.PackSize != "1 l" {
			t.Errorf("pack size = %q, want 1 l", stored.PackSize)
		}
	})

	t.Run("the client's own name survives a barcode the lookup does not know", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Set("offacntrl", &MockOpenFoodFactsAPIController{
			MockGetDataset: func(barcode string) (dbModel.Product, error) {
				return dbModel.Product{}, gorm.ErrRecordNotFound
			},
		})
		env.Ctx.Request = newRawJSONRequest(`{"barcode":"4006381333931","productName":"  Milch  "}`)

		CreateProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var stored dbModel.Product
		if err := env.DB.First(&stored, "barcode = ?", "4006381333931").Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.ProductName != "Milch" {
			t.Errorf("name = %q, want Milch (the whitespace is collapsed)", stored.ProductName)
		}
	})
}
