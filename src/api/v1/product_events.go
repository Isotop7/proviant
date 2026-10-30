// v1 implements version 1 of the proviant API
package v1

import (
	"context"
	"time"

	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	dbModel "github.com/Isotop7/proviant/models/database"
)

// fireProductCreated announces a created product: the activity log entry and
// the product.created webhook. It is the single place both are fired from, so
// a product is announced exactly once no matter which path created it.
func fireProductCreated(repos *database.RepositoryContainer, userID uint, product *dbModel.Product, quantity int) {
	fireActivityLog(repos, userID, dbModel.ActivityActionAdd, product, quantity)

	go func() {
		if ws := controllers.GetWebhookService(); ws != nil {
			ws.FireEvent("product.created", map[string]any{
				"id":          product.ID,
				"productName": product.ProductName,
				"barcode":     product.Barcode,
				"expireAt":    product.ExpireAt,
				"quantity":    product.Quantity,
				"householdId": product.HouseholdID,
			})
		}
	}()
}

// fireQuantityChange logs that packs were added to a product. No product was
// created, so the creation events do not fire — but the feed still shows that
// the pantry changed.
func fireQuantityChange(repos *database.RepositoryContainer, userID uint, product *dbModel.Product, packs int) {
	fireActivityLog(repos, userID, dbModel.ActivityActionQuantityChange, product, packs)
}

// fireActivityLog writes one entry into the household's activity feed.
func fireActivityLog(repos *database.RepositoryContainer, userID uint, action string, product *dbModel.Product, quantity int) {
	go func() {
		householdID, _ := repos.Users.GetUserHouseholdByID(userID)
		userName := ""
		if user, err := repos.Users.GetUserByID(userID); err == nil {
			userName = user.DisplayName
		}
		entry := &dbModel.ActivityLog{
			HouseholdID: householdID,
			UserID:      &userID,
			UserName:    userName,
			Action:      action,
			ProductID:   product.ID,
			ProductName: product.ProductName,
			Quantity:    quantity,
			Timestamp:   time.Now(),
		}
		if repos.ActivityLogs == nil {
			return
		}
		ctxBg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = repos.ActivityLogs.Create(ctxBg, entry)
	}()
}
