package database

import (
	"testing"
	"time"

	dbModel "github.com/Isotop7/proviant/models/database"
)

func TestFilterByEffectiveExpiryWindow(t *testing.T) {
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now().Add(24 * time.Hour)

	inWindow := dbModel.Product{ExpireAt: time.Now()}
	beforeWindow := dbModel.Product{ExpireAt: start.Add(-time.Hour)}
	afterWindow := dbModel.Product{ExpireAt: end.Add(time.Hour)}
	undated := dbModel.Product{}
	// Opened-shelf-life row whose effective expiry fell before the window.
	openedAgo := time.Now().Add(-10 * 24 * time.Hour)
	openedDays := 5
	openedExpired := dbModel.Product{OpenedAt: &openedAgo, DaysAfterOpening: &openedDays}

	filtered := filterByEffectiveExpiryWindow(
		[]dbModel.Product{inWindow, beforeWindow, afterWindow, undated, openedExpired},
		start, end,
	)
	if len(filtered) != 1 || !filtered[0].ExpireAt.Equal(inWindow.ExpireAt) {
		t.Errorf("filterByEffectiveExpiryWindow() = %+v, want only the in-window product", filtered)
	}

	if got := filterByEffectiveExpiryWindow(nil, start, end); len(got) != 0 {
		t.Errorf("filterByEffectiveExpiryWindow(nil) returned %d, want 0", len(got))
	}
}
