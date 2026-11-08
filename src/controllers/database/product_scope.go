package database

import (
	"math"
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// The query fragments below are shared by every product repository, so they are
// package-level functions rather than methods: the household resolution, the
// privacy scope and the effective-expiry prune are one rule each, and a
// repository that built its own copy would be free to drift from it.

// maxDaysAfterOpening is the upper bound enforced on the DaysAfterOpening
// field on PATCH. It is also used to bound the "opened-shelf-life" OR branch
// in effectiveExpiryCandidateScope so the planner can prune candidate rows
// whose opened date is far outside the requested window. Kept as an alias of
// the model constant so the write bound and the query bound cannot drift.
const maxDaysAfterOpening = database.MaxDaysAfterOpening

// effectiveExpiryCandidateScope returns a GORM scope that selects product
// rows whose effective expiry (printed ExpireAt, OR OpenedAt + DaysAfterOpening)
// could fall within [start, end]. The "opened-shelf-life present" branch
// is bounded by maxDaysAfterOpening on the OpenedAt side: any row whose
// OpenedAt is more than maxDaysAfterOpening before start cannot possibly
// fall into the window, even with the smallest positive days-after-opening
// value, so the planner can prune it. The Go caller MUST still filter
// using EffectiveExpireAt() as a correctness safety net (the bound is a
// safe upper, not a tight lower).
//
// The OR clause prevents the planner from using the expire_at index alone,
// so this scope trades index-friendliness for correctness across both
// expiry sources. It is intentional; see the comment on
// Product.EffectiveExpireAt.
func effectiveExpiryCandidateScope(start, end time.Time) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		// A row can only enter the window via the opened branch if
		// OpenedAt + days_after_opening is in [start, end]. The
		// smallest possible days_after_opening is 1, so any row with
		// OpenedAt < start - maxDaysAfterOpening cannot qualify.
		// Similarly OpenedAt > end can never qualify (days >= 1).
		openedFloor := start.AddDate(0, 0, -maxDaysAfterOpening)
		return tx.Where(
			"(expire_at >= ? AND expire_at <= ?) OR "+
				"(opened_at IS NOT NULL AND days_after_opening > 0 "+
				"AND opened_at >= ? AND opened_at <= ?)",
			start, end, openedFloor, end,
		)
	}
}

// filterByEffectiveExpiryWindow returns the subset of products whose
// effective expiry (earlier of printed ExpireAt and OpenedAt+DaysAfterOpening)
// is in the closed window [start, end]. Rows with a zero effective expiry
// are dropped. The slice header is reused to avoid an allocation when
// the input is empty.
func filterByEffectiveExpiryWindow(products []database.Product, start, end time.Time) []database.Product {
	filtered := products[:0]
	for i := range products {
		eff := products[i].EffectiveExpireAt()
		if eff.IsZero() {
			continue
		}
		if eff.Before(start) || eff.After(end) {
			continue
		}
		filtered = append(filtered, products[i])
	}
	return filtered
}

// getUserHouseholdID resolves the household a user belongs to. A user without
// a household is an account in error, so it is reported rather than answered
// with an empty scope.
func getUserHouseholdID(db *gorm.DB, userID uint) (uint, error) {
	var user authentication.User
	if err := db.First(&user, userID).Error; err != nil {
		return 0, err
	}
	if user.HouseholdID == 0 {
		return 0, errors.ErrInvalidUserData
	}
	return user.HouseholdID, nil
}

// privacyScope limits a query to the rows the caller may see: the shared ones
// and their own private ones.
func privacyScope(userID uint) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("(is_private = 0 OR user_id = ?)", userID)
	}
}

// activeProductsQuery returns the query every read of the household's active
// products shares: the household's rows, the ones the caller may see, the rows
// that are not archived.
//
// The list and the count that describes it are both built from it, so the count
// cannot report a different set than the list — a count that leaked another
// household's rows or another member's private ones would be a leak of its own.
//
// The query it returns is single-use: it carries one statement, so every
// terminal call (a Find, a Count) needs a query of its own.
func activeProductsQuery(db *gorm.DB, userID uint) (*gorm.DB, error) {
	householdID, err := getUserHouseholdID(db, userID)
	if err != nil {
		return nil, err
	}

	return db.
		Scopes(privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull), nil
}

// archivedProductsQuery is the archived counterpart of activeProductsQuery: the
// same household and the same privacy rules, but the rows that are archived.
func archivedProductsQuery(db *gorm.DB, userID uint) (*gorm.DB, error) {
	householdID, err := getUserHouseholdID(db, userID)
	if err != nil {
		return nil, err
	}

	return db.Unscoped().
		Scopes(privacyScope(userID)).
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID), nil
}

// pageWindow returns the rows a page covers: the rows after the offset, at
// most limit of them. A limit of zero means every remaining row, an offset
// beyond the last row means an empty page.
func pageWindow(rows []database.Product, limit, offset int) []database.Product {
	if offset > 0 {
		if offset >= len(rows) {
			return []database.Product{}
		}
		rows = rows[offset:]
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// applyPage limits a query to the window a page covers.
//
// An offset always carries a limit: MySQL has no OFFSET without a LIMIT and
// rejects the statement, while the SQLite driver rewrites a bare OFFSET into
// "LIMIT -1 OFFSET n" on its own. A limit of zero means "every row", so the
// offset alone becomes the largest limit the database can express.
func applyPage(query *gorm.DB, limit, offset int) *gorm.DB {
	if limit <= 0 && offset > 0 {
		limit = math.MaxInt
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	return query
}

// bulkQueryChunkSize bounds the number of placeholders per IN clause. SQLite
// builds have historically capped bound variables at 999, so a full import's
// 5000 barcodes cannot go into one query.
const bulkQueryChunkSize = 500

// chunkStrings splits values into consecutive slices of at most size elements.
func chunkStrings(values []string, size int) [][]string {
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := min(start+size, len(values))
		chunks = append(chunks, values[start:end])
	}
	return chunks
}
