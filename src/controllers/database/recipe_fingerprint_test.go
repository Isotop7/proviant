package database

import (
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/testutil"
)

// The recipe suggestion cache stores product IDs that the client turns into an
// irreversible "cook this" action, so the fingerprint must move whenever the
// matchable product set changes — including for edits that leave the expiring
// set, and therefore every other cache-key input, untouched.
func TestGetProductSetFingerprint(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	repo := NewProductRepository(db)

	fingerprint := func() string {
		t.Helper()
		got, err := repo.GetProductSetFingerprint(household.ID)
		if err != nil {
			t.Fatalf("GetProductSetFingerprint() error = %v", err)
		}
		return got
	}

	baseline := fingerprint()
	if again := fingerprint(); again != baseline {
		t.Errorf("fingerprint is not stable for an unchanged set: %q then %q", baseline, again)
	}

	t.Run("Add", func(t *testing.T) {
		before := fingerprint()
		testutil.CreateTestProduct(db, household.ID)
		if after := fingerprint(); after == before {
			t.Error("fingerprint unchanged after adding a product")
		}
	})

	t.Run("Rename", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID)
		if err := db.Model(product).Update("product_name", "Renamed Product").Error; err != nil {
			t.Fatalf("rename failed: %v", err)
		}
		before := fingerprint()
		if err := db.Model(product).Update("product_name", "Renamed Again").Error; err != nil {
			t.Fatalf("second rename failed: %v", err)
		}
		if after := fingerprint(); after == before {
			t.Error("fingerprint unchanged after renaming a product, so a stale match could still be served")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		product := testutil.CreateTestProduct(db, household.ID)
		before := fingerprint()
		if err := db.Delete(product).Error; err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		if after := fingerprint(); after == before {
			t.Error("fingerprint unchanged after deleting a product")
		}
	})
}

// The fingerprint must survive writes that cannot change what a recipe matched.
// Every product write moves updated_at, so folding that column in made each
// request in an actively used household miss the recipe cache and pay a full
// provider fan-out.
func TestGetProductSetFingerprint_IgnoresQuantityChanges(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	repo := NewProductRepository(db)
	product := testutil.CreateTestProduct(db, household.ID)

	before, err := repo.GetProductSetFingerprint(household.ID)
	if err != nil {
		t.Fatalf("GetProductSetFingerprint() error = %v", err)
	}

	if err := db.Model(product).Update("amount", 7).Error; err != nil {
		t.Fatalf("quantity update failed: %v", err)
	}

	after, err := repo.GetProductSetFingerprint(household.ID)
	if err != nil {
		t.Fatalf("GetProductSetFingerprint() error = %v", err)
	}
	if after != before {
		t.Errorf("fingerprint changed after a quantity update: %q then %q — every product write would invalidate the recipe cache", before, after)
	}
}

// A private product must not contribute: the fingerprint has to describe the
// same rows GetProductsByHousehold returns, or changes the matcher never sees
// would needlessly invalidate the cache.
func TestGetProductSetFingerprint_IgnoresPrivateProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	repo := NewProductRepository(db)

	baseline, err := repo.GetProductSetFingerprint(household.ID)
	if err != nil {
		t.Fatalf("GetProductSetFingerprint() error = %v", err)
	}

	private := testutil.CreateTestProduct(db, household.ID)
	if err := db.Model(private).Update("is_private", true).Error; err != nil {
		t.Fatalf("marking private failed: %v", err)
	}

	after, err := repo.GetProductSetFingerprint(household.ID)
	if err != nil {
		t.Fatalf("GetProductSetFingerprint() error = %v", err)
	}
	if after != baseline {
		t.Errorf("private product changed the fingerprint: %q then %q", baseline, after)
	}
}

// Ranking is expiry-proximity-first and runs only at match time — ExpiryPoints
// is json:"-" — so a cached entry cannot be re-sorted on read. An expiry edit
// that moves a product into a different ranking bucket changes no other
// cache-key input, so without expiry in the fingerprint the stale order would be
// served for the whole TTL.
func TestGetProductSetFingerprint_TracksExpiryBucket(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	repo := NewProductRepository(db)
	product := testutil.CreateTestProduct(db, household.ID)

	fingerprint := func() string {
		t.Helper()
		got, err := repo.GetProductSetFingerprint(household.ID)
		if err != nil {
			t.Fatalf("GetProductSetFingerprint() error = %v", err)
		}
		return got
	}

	// Anchored on the real clock because the repository buckets against
	// time.Now() itself. The half-day offset parks every timestamp in the
	// middle of its bucket, so the assertions hold no matter when in the day
	// they run and cannot straddle a boundary.
	offset := func(days int) time.Time {
		return time.Now().Add(time.Duration(days)*24*time.Hour + 12*time.Hour)
	}

	if err := db.Model(product).Update("expire_at", offset(6)).Error; err != nil {
		t.Fatalf("setting expire_at failed: %v", err)
	}
	far := fingerprint()

	if err := db.Model(product).Update("expire_at", offset(2)).Error; err != nil {
		t.Fatalf("moving expire_at failed: %v", err)
	}
	near := fingerprint()

	// A different ranking bucket changes the score and must rekey.
	if near == far {
		t.Error("fingerprint unchanged after the expiry moved to a different ranking bucket, so the stale expiry ranking would be served for a full cache TTL")
	}

	// An edit that stays inside one bucket cannot change any product's score and
	// must not invalidate: bucket granularity is what keeps quantity- and
	// scan-adjacent writes from rekeying the cache.
	if err := db.Model(product).Update("expire_at", offset(2).Add(time.Hour)).Error; err != nil {
		t.Fatalf("same-bucket expire_at update failed: %v", err)
	}
	if after := fingerprint(); after != near {
		t.Errorf("fingerprint changed for an expiry edit inside one ranking bucket: %q then %q", near, after)
	}
}
