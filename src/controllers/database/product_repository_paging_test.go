package database

import (
	"fmt"
	"strings"
	"testing"
	"time"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// seedPagedProducts creates products of the household that share their name,
// so a column sort has to break the ties itself.
func seedPagedProducts(t *testing.T, db *gorm.DB, householdID, userID uint, count int) []*dbModel.Product {
	t.Helper()
	products := make([]*dbModel.Product, 0, count)
	for i := range count {
		product := dbModel.Product{
			ProductName: "Milk",
			Barcode:     fmt.Sprintf("400000000%04d", i),
			HouseholdID: householdID,
			UserID:      userID,
			ExpireAt:    time.Now().AddDate(0, 0, i+1),
		}
		if err := db.Create(&product).Error; err != nil {
			t.Fatalf("create product: %v", err)
		}
		products = append(products, &product)
	}
	return products
}

// TestApplyPageCarriesALimitWithAnOffset pins the statement a MariaDB
// deployment runs: MySQL has no OFFSET without a LIMIT and rejects the
// statement, while the SQLite driver rewrites a bare OFFSET into
// "LIMIT -1 OFFSET n" on its own. A page that skips rows therefore always
// carries a limit, and the statement is built against the MySQL dialect to
// keep the SQLite driver from hiding the difference.
func TestApplyPageCarriesALimitWithAnOffset(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DriverName:                "mysql",
		DSN:                       "user:password@tcp(127.0.0.1:3306)/proviant",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open the dry-run mysql connection: %v", err)
	}

	var products []dbModel.Product
	statement := applyPage(db.Model(&dbModel.Product{}).Order("id ASC"), 0, 50)
	if findErr := statement.Find(&products).Error; findErr != nil {
		t.Fatalf("find: %v", findErr)
	}

	built := statement.Statement.SQL.String()
	if !strings.Contains(built, "LIMIT") {
		t.Errorf("statement = %q, want a LIMIT next to the OFFSET — MySQL rejects an OFFSET without one", built)
	}
	if !strings.Contains(built, "OFFSET") {
		t.Errorf("statement = %q, want the OFFSET of the page", built)
	}
}

// TestSearchProductsPagesByEffectiveExpiry pins the paging of a search ordered
// by the effective expiry: the matches are resolved before the page is cut out,
// because the effective expiry has no sortable column.
func TestSearchProductsPagesByEffectiveExpiry(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	expiring := make([]*dbModel.Product, 0, 3)
	for i := range 3 {
		product := dbModel.Product{
			ProductName: "Milk",
			Barcode:     fmt.Sprintf("430000000%04d", i),
			HouseholdID: household.ID,
			UserID:      user.ID,
			ExpireAt:    time.Now().AddDate(0, 0, i+1),
		}
		if err := db.Create(&product).Error; err != nil {
			t.Fatalf("create product: %v", err)
		}
		expiring = append(expiring, &product)
	}
	repo := NewProductRepository(db)

	first, err := repo.SearchProducts(ProductName, "Milk", SortEffectiveExpiry, "asc", user.ID, 2, 0)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	second, err := repo.SearchProducts(ProductName, "Milk", SortEffectiveExpiry, "asc", user.ID, 2, 2)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}

	if len(first) != 2 || len(second) != 1 {
		t.Fatalf("pages hold %d and %d matches, want 2 and 1", len(first), len(second))
	}
	want := []uint{expiring[0].ID, expiring[1].ID, expiring[2].ID}
	got := []uint{first[0].ID, first[1].ID, second[0].ID}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("match %d = %d, want %d — the page follows the effective expiry", i, got[i], want[i])
		}
	}
}

// TestGetUserProductsBulkSortedPages pins the paging of a column sort: the
// pages carry different rows and together they cover the whole set.
func TestGetUserProductsBulkSortedPages(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	seedPagedProducts(t, db, household.ID, user.ID, 5)
	repo := NewProductRepository(db)

	seen := map[uint]bool{}
	for offset := 0; offset < 10; offset += 2 {
		page, err := repo.GetUserProductsBulkSorted(user.ID, 2, offset, "product_name", "asc")
		if err != nil {
			t.Fatalf("GetUserProductsBulkSorted() error = %v", err)
		}
		for _, product := range page {
			if seen[product.ID] {
				t.Fatalf("product %d came back on two pages", product.ID)
			}
			seen[product.ID] = true
		}
		if len(page) < 2 {
			break
		}
	}

	if len(seen) != 5 {
		t.Errorf("the walk collected %d products, want all 5", len(seen))
	}
}

// TestGetUserProductsBulkSortedKeepsTiesInIdOrder pins the tiebreak of a
// column sort: products that share the sort key keep the order of their ids,
// so a walk over them reads every one of them exactly once.
func TestGetUserProductsBulkSortedKeepsTiesInIdOrder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	products := seedPagedProducts(t, db, household.ID, user.ID, 4)
	repo := NewProductRepository(db)

	first, err := repo.GetUserProductsBulkSorted(user.ID, 2, 0, "product_name", "asc")
	if err != nil {
		t.Fatalf("GetUserProductsBulkSorted() error = %v", err)
	}
	second, err := repo.GetUserProductsBulkSorted(user.ID, 2, 2, "product_name", "asc")
	if err != nil {
		t.Fatalf("GetUserProductsBulkSorted() error = %v", err)
	}

	got := []uint{first[0].ID, first[1].ID, second[0].ID, second[1].ID}
	for i, want := range products {
		if got[i] != want.ID {
			t.Errorf("product %d of the walk = %d, want %d — the ties keep the order of their ids", i, got[i], want.ID)
		}
	}
}

// TestGetActiveProductsCountMatchesTheList pins that the count describes the
// same set the list returns: a client compares the rows it walked with the
// total and expects them to be the same rows.
func TestGetActiveProductsCountMatchesTheList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	seedPagedProducts(t, db, household.ID, user.ID, 5)

	otherHousehold := testutil.CreateTestHousehold(db, 0)
	otherUser := testutil.CreateTestUser(db, otherHousehold.ID)
	seedPagedProducts(t, db, otherHousehold.ID, otherUser.ID, 3)

	repo := NewProductRepository(db)

	count, err := repo.GetActiveProductsCount(user.ID)
	if err != nil {
		t.Fatalf("GetActiveProductsCount() error = %v", err)
	}
	rows, err := repo.GetUserProductsBulkSorted(user.ID, 0, 0, SortEffectiveExpiry, "asc")
	if err != nil {
		t.Fatalf("GetUserProductsBulkSorted() error = %v", err)
	}
	if count != len(rows) {
		t.Errorf("count = %d, want the %d rows the list returns", count, len(rows))
	}
	if count != 5 {
		t.Errorf("count = %d, want the 5 products of the household", count)
	}
}

// TestGetArchivedProductsCountMatchesTheList pins the count of the archived
// list against the rows it returns.
func TestGetArchivedProductsCountMatchesTheList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	for i := range 3 {
		product := dbModel.Product{
			ProductName: "Milk",
			Barcode:     fmt.Sprintf("410000000%04d", i),
			HouseholdID: household.ID,
			UserID:      user.ID,
		}
		if err := db.Create(&product).Error; err != nil {
			t.Fatalf("create product: %v", err)
		}
		if err := db.Delete(&product).Error; err != nil {
			t.Fatalf("archive product: %v", err)
		}
	}

	repo := NewProductRepository(db)

	count, err := repo.GetArchivedProductsCount(user.ID)
	if err != nil {
		t.Fatalf("GetArchivedProductsCount() error = %v", err)
	}
	rows, err := repo.GetUserArchivedProductsBulk(user.ID, 0, 0)
	if err != nil {
		t.Fatalf("GetUserArchivedProductsBulk() error = %v", err)
	}
	if count != len(rows) {
		t.Errorf("count = %d, want the %d rows the archived list returns", count, len(rows))
	}
}

// TestCountSearchProductsMatchesTheSearch pins the count of the search against
// the rows it returns.
func TestCountSearchProductsMatchesTheSearch(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	seedPagedProducts(t, db, household.ID, user.ID, 4)
	other := dbModel.Product{
		ProductName: "Bread",
		Barcode:     "4200000000000",
		HouseholdID: household.ID,
		UserID:      user.ID,
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	repo := NewProductRepository(db)

	count, err := repo.CountSearchProducts(ProductName, "Milk", user.ID)
	if err != nil {
		t.Fatalf("CountSearchProducts() error = %v", err)
	}
	rows, err := repo.SearchProducts(ProductName, "Milk", "product_name", "asc", user.ID, 0, 0)
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if count != len(rows) {
		t.Errorf("count = %d, want the %d rows the search returns", count, len(rows))
	}
	if count != 4 {
		t.Errorf("count = %d, want the 4 matches", count)
	}
}

// TestProductRepository_CountDoesNotLoadTheRows pins that the count is a count
// in SQL: answering "how many products are there" must not cost the rows it
// counts.
func TestProductRepository_CountDoesNotLoadTheRows(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	seedPagedProducts(t, db, household.ID, user.ID, 20)

	counter := testutil.NewQueryCounter()
	counting := db.Session(&gorm.Session{Logger: counter})
	repo := NewProductRepository(counting)

	if _, err := repo.GetActiveProductsCount(user.ID); err != nil {
		t.Fatalf("GetActiveProductsCount() error = %v", err)
	}

	statements := counter.Statements()
	if len(statements) == 0 {
		t.Fatal("no statement was recorded")
	}
	counted := false
	for _, statement := range statements {
		if strings.Contains(statement, "count(") {
			counted = true
		}
		if strings.Contains(statement, "SELECT * FROM `products`") {
			t.Errorf("the count loaded the rows it counts: %s", statement)
		}
	}
	if !counted {
		t.Errorf("no count statement was run: %v", statements)
	}
}
