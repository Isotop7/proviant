package database

import (
	"fmt"
	"sort"
	"time"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// ProductDigestRepositoryInterface is the selection the notifications need:
// the products the mail digest lists and the samples the consumption
// statistics are built from.
type ProductDigestRepositoryInterface interface {
	GetExpiringProductsForMailDigest(householdID uint) (MailDigestProductGroup, error)
	GetConsumedSamples(householdID, userID uint, barcode, name string, since time.Time) ([]database.Product, error)
}

var _ ProductDigestRepositoryInterface = (*ProductDigestRepository)(nil)

// ProductDigestRepository answers the selection queries the notifications and
// the consumption statistics read.
type ProductDigestRepository struct {
	DB *gorm.DB
}

func NewProductDigestRepository(db *gorm.DB) *ProductDigestRepository {
	return &ProductDigestRepository{DB: db}
}

type MailDigestProductGroup struct {
	Today    []database.Product
	ThisWeek []database.Product
	NextWeek []database.Product
}

func (r *ProductDigestRepository) GetExpiringProductsForMailDigest(householdID uint) (MailDigestProductGroup, error) {
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfToday := startOfToday.AddDate(0, 0, 1).Add(-time.Nanosecond)
	endOfWeek := startOfToday.AddDate(0, 0, 7).Add(-time.Nanosecond)
	endOfNextWeek := startOfToday.AddDate(0, 0, 14).Add(-time.Nanosecond)

	var allProducts []database.Product
	// SQL filter: either the printed expiry is within the 2-week window,
	// or the row has an opened-shelf-life rule. Final bucketing is done
	// in Go using EffectiveExpireAt() to keep the query portable across
	// SQLite and MariaDB.
	if err := r.DB.
		Where("household_id = ? AND deleted_at IS NULL AND is_private = 0", householdID).
		Scopes(effectiveExpiryCandidateScope(startOfToday, endOfNextWeek)).
		Find(&allProducts).Error; err != nil {
		return MailDigestProductGroup{}, err
	}

	var group MailDigestProductGroup
	for i := range allProducts {
		p := &allProducts[i]
		eff := p.EffectiveExpireAt()
		if eff.IsZero() || eff.Before(startOfToday) {
			continue
		}
		switch {
		case !eff.After(endOfToday):
			group.Today = append(group.Today, *p)
		case !eff.After(endOfWeek):
			group.ThisWeek = append(group.ThisWeek, *p)
		default:
			group.NextWeek = append(group.NextWeek, *p)
		}
	}

	sortEffectiveExpire := func(s []database.Product) {
		sort.Slice(s, func(i, j int) bool {
			ti, tj := s[i].EffectiveExpireAt(), s[j].EffectiveExpireAt()
			if ti.IsZero() && tj.IsZero() {
				return false
			}
			if ti.IsZero() {
				return false
			}
			if tj.IsZero() {
				return true
			}
			return ti.Before(tj)
		})
	}
	sortEffectiveExpire(group.Today)
	sortEffectiveExpire(group.ThisWeek)
	sortEffectiveExpire(group.NextWeek)

	return group, nil
}

func (r *ProductDigestRepository) GetConsumedSamples(householdID, userID uint, barcode, name string, since time.Time) ([]database.Product, error) {
	if barcode == "" && name == "" {
		return nil, fmt.Errorf("GetConsumedSamples: barcode and name are both empty")
	}

	query := r.DB.Unscoped().
		Select("id, product_name, barcode, quantity, deleted_at, is_private, user_id, removal_reason").
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID).
		Where("deleted_at >= ?", since).
		Where("removal_reason = ?", database.RemovalReasonConsumed).
		Where("(is_private = ? OR user_id = ?)", false, userID)

	if barcode != "" {
		query = query.Where("barcode = ?", barcode)
	} else {
		query = query.Where("product_name = ?", name)
	}

	var products []database.Product
	if err := query.Order("deleted_at ASC").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}
