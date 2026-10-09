package services

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func newTestConsumptionService(db *gorm.DB) *ConsumptionService {
	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(db, nil)
	return NewConsumptionService(repos, &logger)
}

func TestConsumptionService_ComputeConsumptionRate(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestConsumptionService(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	now := time.Now()

	t.Run("no samples returns zero rate", func(t *testing.T) {
		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, "0000000000000", "Unknown")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rate.HasEstimate {
			t.Errorf("HasEstimate = true, want false")
		}
		if rate.SampleCount != 0 {
			t.Errorf("SampleCount = %d, want 0", rate.SampleCount)
		}
	})

	t.Run("single sample returns no estimate", func(t *testing.T) {
		barcode := "1111111111111"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Milk", "L", 1, now.Add(-2*24*time.Hour))
		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "Milk")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rate.HasEstimate {
			t.Errorf("HasEstimate = true, want false (only 1 sample)")
		}
		if rate.SampleCount != 1 {
			t.Errorf("SampleCount = %d, want 1", rate.SampleCount)
		}
	})

	t.Run("two samples 7 days apart with span >= 7 days produce a 2 L per week rate", func(t *testing.T) {
		barcode := "2222222222222"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Milk", "L", 1, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Milk", "L", 1, now.Add(-7*24*time.Hour))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "Milk")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rate.HasEstimate {
			t.Fatalf("HasEstimate = false, want true")
		}
		if rate.Unit != "L" {
			t.Errorf("Unit = %q, want L", rate.Unit)
		}
		expectedPerWeek := 2.0
		if rate.PerWeek < expectedPerWeek-0.01 || rate.PerWeek > expectedPerWeek+0.01 {
			t.Errorf("PerWeek = %f, want ~%f", rate.PerWeek, expectedPerWeek)
		}
		if rate.Display == "" {
			t.Errorf("Display = empty, want non-empty")
		}
		if rate.TemplateMessage == "" {
			t.Errorf("TemplateMessage = empty, want non-empty")
		}
	})

	t.Run("two samples 14 days apart with span >= 7 days produce a 1 L per week rate", func(t *testing.T) {
		barcode := "2222222222299"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Yogurt", "pcs", 1, now.Add(-21*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Yogurt", "pcs", 1, now.Add(-7*24*time.Hour))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "Yogurt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rate.HasEstimate {
			t.Fatalf("HasEstimate = false, want true")
		}
		expectedPerWeek := 1.0
		if rate.PerWeek < expectedPerWeek-0.01 || rate.PerWeek > expectedPerWeek+0.01 {
			t.Errorf("PerWeek = %f, want ~%f (1 unit over 14 days)", rate.PerWeek, expectedPerWeek)
		}
	})

	t.Run("two samples within the same day produce no estimate (min span gate)", func(t *testing.T) {
		// Two same-day consumptions must NOT yield a stable per-week rate.
		// Without the span gate this would produce perWeek ~ 14 (highly
		// inflated).
		barcode := "2222222222223"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Eggs", "pcs", 1, now.Add(-1*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Eggs", "pcs", 1, now.Add(-30*time.Minute))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "Eggs")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rate.HasEstimate {
			t.Errorf("HasEstimate = true, want false (same-day samples must not produce per-week)")
		}
		if rate.PerWeek != 0 {
			t.Errorf("PerWeek = %f, want 0 when no estimate", rate.PerWeek)
		}
	})

	t.Run("sub-hour skew on a true 7-day span still produces an estimate (DST safety)", func(t *testing.T) {
		// 7 days minus 1 hour = 167 hours. Without the math.Round fix
		// this truncates to 6 days and silently drops the estimate.
		barcode := "2222222222224"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Bread", "pcs", 1, now.Add(-(7*24*time.Hour + time.Hour)))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Bread", "pcs", 1, now.Add(-time.Hour))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "Bread")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rate.HasEstimate {
			t.Errorf("HasEstimate = false, want true (true 7-day span with sub-hour skew must not be truncated)")
		}
	})

	t.Run("zero amount counts as 1", func(t *testing.T) {
		barcode := "3333333333333"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Eggs", "pcs", 0, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Eggs", "pcs", 0, now.Add(-7*24*time.Hour))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "Eggs")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rate.HasEstimate {
			t.Errorf("HasEstimate = false, want true")
		}
		if rate.PerWeek < 1.99 || rate.PerWeek > 2.01 {
			t.Errorf("PerWeek = %f, want ~2.0", rate.PerWeek)
		}
	})

	t.Run("dominant unit wins on count, most recent on tie", func(t *testing.T) {
		// 2x kg (older) and 1x pcs (newer). kg has the higher count
		// and should win.
		barcode := "4444444444444"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Butter", "kg", 1, now.Add(-21*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Butter", "kg", 1, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Butter", "pcs", 1, now.Add(-7*24*time.Hour))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "Butter")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rate.Unit != "kg" {
			t.Errorf("Unit = %q, want kg (most-frequent)", rate.Unit)
		}

		// Now flip the counts: 1x kg, 2x pcs. pcs should win on count.
		barcode2 := "4444444444445"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode2, "Butter2", "kg", 1, now.Add(-21*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode2, "Butter2", "pcs", 1, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode2, "Butter2", "pcs", 1, now.Add(-7*24*time.Hour))

		rate2, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode2, "Butter2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rate2.Unit != "pcs" {
			t.Errorf("Unit = %q, want pcs (count 2 vs 1)", rate2.Unit)
		}
	})

	t.Run("on a true tie, most recent sample's unit wins", func(t *testing.T) {
		// 1x kg (older) and 1x pcs (newer) — true tie, pcs should win.
		barcode := "4444444444446"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "ButterTie", "kg", 1, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "ButterTie", "pcs", 1, now.Add(-7*24*time.Hour))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "ButterTie")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rate.Unit != "pcs" {
			t.Errorf("Unit = %q, want pcs (more recent on tie)", rate.Unit)
		}
	})

	t.Run("empty barcode falls back to name", func(t *testing.T) {
		name := "Loose Tomatoes"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, "", name, "kg", 1, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, "", name, "kg", 1, now.Add(-7*24*time.Hour))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, "", name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rate.HasEstimate {
			t.Errorf("HasEstimate = false, want true (name fallback)")
		}
		if rate.SampleCount != 2 {
			t.Errorf("SampleCount = %d, want 2", rate.SampleCount)
		}
	})

	t.Run("90-day window excludes older samples", func(t *testing.T) {
		barcode := "5555555555555"
		old := now.AddDate(0, 0, -(util.ConsumptionHistoryWindowDays + 10))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Yogurt", "pcs", 1, old)
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Yogurt", "pcs", 1, now.Add(-2*24*time.Hour))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "Yogurt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rate.SampleCount != 1 {
			t.Errorf("SampleCount = %d, want 1 (old sample excluded)", rate.SampleCount)
		}
		if rate.HasEstimate {
			t.Errorf("HasEstimate = true, want false")
		}
	})

	t.Run("privacy: other user's private samples of same barcode are excluded", func(t *testing.T) {
		// other user has private samples of barcode X. The current user
		// computing the rate for the same barcode must not see them.
		other := testutil.CreateTestUser(db, household.ID)
		barcode := "6666666666666"
		testutil.SeedConsumedProduct(t, db, household.ID, other.ID, true, barcode, "Tea", "pcs", 1, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, other.ID, true, barcode, "Tea", "pcs", 1, now.Add(-7*24*time.Hour))

		rate, err := svc.ComputeConsumptionRate(household.ID, user.ID, barcode, "Tea")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rate.SampleCount != 0 {
			t.Errorf("SampleCount = %d, want 0 (other user's private samples must be hidden)", rate.SampleCount)
		}
		if rate.HasEstimate {
			t.Errorf("HasEstimate = true, want false (no visible samples)")
		}

		// Other user querying for the same barcode should see their own.
		rateOther, err := svc.ComputeConsumptionRate(household.ID, other.ID, barcode, "Tea")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rateOther.SampleCount != 2 {
			t.Errorf("other user SampleCount = %d, want 2 (own private samples visible)", rateOther.SampleCount)
		}
	})
}

func TestConsumptionService_ComputeRestockSuggestionFromProduct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := newTestConsumptionService(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	now := time.Now()

	t.Run("uses rate when estimate available", func(t *testing.T) {
		barcode := "6666666666666"
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Yogurt", "pcs", 1, now.Add(-14*24*time.Hour))
		testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, barcode, "Yogurt", "pcs", 1, now.Add(-7*24*time.Hour))
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.Barcode = barcode
		product.ProductName = "Yogurt"
		product.Unit = "pcs"
		db.Save(product)

		sug := svc.ComputeRestockSuggestionFromProduct(household.ID, user.ID, product)
		if !sug.HasEstimate {
			t.Errorf("HasEstimate = false, want true")
		}
		if sug.Source != util.ConsumptionSourceRate {
			t.Errorf("Source = %q, want %q", sug.Source, util.ConsumptionSourceRate)
		}
		if sug.SuggestedQty < 1 {
			t.Errorf("SuggestedQty = %d, want >= 1", sug.SuggestedQty)
		}
		if sug.Display == "" {
			t.Errorf("Display = empty, want non-empty")
		}
		if sug.TemplateMessage == "" {
			t.Errorf("TemplateMessage = empty, want non-empty")
		}
		if sug.PerWeekDisplay == "" {
			t.Errorf("PerWeekDisplay = empty, want non-empty (server pre-formats the rate)")
		}
	})

	t.Run("falls back to min stock when no estimate and amount < minStock", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.Barcode = "7777777777777"
		product.ProductName = "Flour"
		product.Unit = "kg"
		product.Amount = 1
		product.MinStockAmount = 3
		db.Save(product)

		sug := svc.ComputeRestockSuggestionFromProduct(household.ID, user.ID, product)
		if sug.Source != util.ConsumptionSourceMinStock {
			t.Errorf("Source = %q, want %q", sug.Source, util.ConsumptionSourceMinStock)
		}
		if sug.SuggestedQty != 2 {
			t.Errorf("SuggestedQty = %d, want 2 (minStock - amount)", sug.SuggestedQty)
		}
		if sug.PerWeekDisplay != "" {
			t.Errorf("PerWeekDisplay = %q, want empty for min_stock branch", sug.PerWeekDisplay)
		}
	})

	t.Run("min stock already met returns none (no misleading suggestion)", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.Barcode = "8888888888888"
		product.ProductName = "Sugar"
		product.Unit = "kg"
		product.Amount = 5
		product.MinStockAmount = 5
		db.Save(product)

		sug := svc.ComputeRestockSuggestionFromProduct(household.ID, user.ID, product)
		if sug.Source != util.ConsumptionSourceNone {
			t.Errorf("Source = %q, want %q (minimum already met, no restock needed)", sug.Source, util.ConsumptionSourceNone)
		}
		if sug.SuggestedQty != 0 {
			t.Errorf("SuggestedQty = %d, want 0", sug.SuggestedQty)
		}
		if sug.HasEstimate {
			t.Errorf("HasEstimate = true, want false")
		}
	})

	t.Run("no estimate and no min stock returns none", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.Barcode = "9999999999999"
		product.ProductName = "Salt"
		product.Unit = "g"
		product.Amount = 5
		product.MinStockAmount = 0
		db.Save(product)

		sug := svc.ComputeRestockSuggestionFromProduct(household.ID, user.ID, product)
		if sug.Source != util.ConsumptionSourceNone {
			t.Errorf("Source = %q, want %q", sug.Source, util.ConsumptionSourceNone)
		}
		if sug.SuggestedQty != 0 {
			t.Errorf("SuggestedQty = %d, want 0", sug.SuggestedQty)
		}
	})
}
