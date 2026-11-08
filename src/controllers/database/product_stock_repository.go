package database

import (
	stderrors "errors"
	"strings"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProductStockRepositoryInterface is the stock side of a product: the quantity
// delta, the consumption and the waste. Every method here moves stock, and
// none of them changes the product's identity or its metadata.
type ProductStockRepositoryInterface interface {
	UpdateProductQuantity(productID uint, userID uint, delta int, mutationID *string) (bool, bool, error)
	ConsumeProduct(productID, userID uint) error
	ConsumeProductPartial(product *database.Product, quantity int, mutationID *string) (int, bool, bool, error)
	WasteProduct(productID, userID uint, cause string) error
	WasteProductPartial(product *database.Product, quantity int, mutationID *string, cause string) (int, bool, bool, error)
	BulkConsumeProducts(productIDs []uint, userID uint) []BulkOperationError
	BulkWasteProducts(productIDs []uint, userID uint, cause string) []BulkOperationError
}

var _ ProductStockRepositoryInterface = (*ProductStockRepository)(nil)

// ProductStockRepository moves stock: a quantity delta, a consumption and a
// waste.
type ProductStockRepository struct {
	DB *gorm.DB
}

func NewProductStockRepository(db *gorm.DB) *ProductStockRepository {
	return &ProductStockRepository{DB: db}
}

// errMutationAlreadyClaimed unwinds the transaction that lost the race for a
// clientMutationId, so the caller can report a replay instead of an error.
var errMutationAlreadyClaimed = stderrors.New("client mutation id already applied")

// errMutationClaimRefused reports a conflict on the claim insert that no claim
// row explains — MySQL's INSERT IGNORE swallows a malformed value the same way
// it swallows a duplicate, and treating that as a replay would drop a real delta.
var errMutationClaimRefused = stderrors.New("client mutation id was not claimable and no claim exists")

// UpdateProductQuantity applies delta to the product and reports it as
// (deleted, applied, err). applied is false when the mutation id was already
// applied, so callers can skip side effects a replay must not repeat twice.
// Claims outlive this call and expire through the claim sweep in proviant.go,
// which is the only thing that bounds the table: pruning per product here would
// never run for a product whose last mutation was its only claim.
// The count stays inside the range a product may hold: the delta is clamped
// there, so no path can write a count the create and the patch would refuse.
func (r *ProductStockRepository) UpdateProductQuantity(productID uint, userID uint, delta int, mutationID *string) (bool, applied bool, err error) {
	if productID == 0 {
		return false, false, gorm.ErrNotImplemented
	}

	// One column: the count moves through the statement below, so the row is
	// read here only to answer "is this the caller's product".
	var dbProduct database.Product
	getError := r.DB.Unscoped().Select("household_id").First(&dbProduct, productID)
	if getError.Error != nil {
		return false, false, getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return false, false, userErr.Error
	}

	if dbProduct.HouseholdID != user.HouseholdID {
		return false, false, errors.ErrMismatcherUserID
	}

	if mutationID != nil && strings.TrimSpace(*mutationID) == "" {
		mutationID = nil
	}

	// The claim and the count share one transaction: a failure between
	// the two statements would leave the delta claimed and unapplied, and
	// every retry of it would be deduped into silence.
	// A conflict is only believed once the claim row behind it exists:
	// MariaDB renders this as INSERT IGNORE, which reports a value it
	// refused to store as zero rows exactly like a duplicate.
	moveErr := r.DB.Transaction(func(tx *gorm.DB) error {
		if mutationID != nil {
			claim := database.ProductMutationClaim{ProductID: productID, MutationID: *mutationID}
			claimResult := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
			if claimResult.Error != nil {
				return claimResult.Error
			}
			if claimResult.RowsAffected == 0 {
				var claimed database.ProductMutationClaim
				if err := tx.First(&claimed, "product_id = ? AND mutation_id = ?", productID, *mutationID).Error; err != nil {
					return errMutationClaimRefused
				}
				return errMutationAlreadyClaimed
			}
		}
		return tx.Unscoped().Model(&database.Product{}).Where("id = ?", productID).
			Updates(map[string]any{
				// The count stays inside the range a product may hold: a delta
				// that would push it below zero lands on zero, one that would
				// push it past the maximum lands on the maximum. The bounds are
				// spelled as a CASE and not as MIN or LEAST because no scalar
				// minimum is portable across SQLite, MySQL and PostgreSQL.
				"quantity": gorm.Expr(
					"CASE WHEN quantity + ? < 0 THEN 0 WHEN quantity + ? > ? THEN ? ELSE quantity + ? END",
					delta, delta, util.MaxProductQuantity, util.MaxProductQuantity, delta),
			}).Error
	})
	if stderrors.Is(moveErr, errMutationAlreadyClaimed) {
		return false, false, nil
	}
	if moveErr != nil {
		return false, false, moveErr
	}

	var current database.Product
	if readErr := r.DB.Unscoped().First(&current, productID).Error; readErr != nil {
		if stderrors.Is(readErr, gorm.ErrRecordNotFound) {
			return true, true, nil
		}
		return false, false, readErr
	}
	if current.Quantity > 0 {
		return false, true, nil
	}

	// The guard belongs in the DELETE as well as in the read above: an
	// increment landing between that read and this statement would otherwise be
	// lost along with the row, leaving it archived with a positive count. Only a
	// row still at zero is eligible, so a concurrent increment cancels the delete
	// instead of being discarded.
	deleteResult := r.DB.Unscoped().
		Where("id = ? AND quantity <= 0", productID).
		Delete(&database.Product{})
	if deleteResult.Error != nil {
		return true, true, deleteResult.Error
	}
	if deleteResult.RowsAffected > 0 {
		return true, true, nil
	}
	// The guard refused the row, which means one of two things. A concurrent
	// increment raised the count and the product survives. Or a concurrent call
	// already hard-deleted the row, in which case this delta was part of the same
	// zeroing and the product really is gone — reading back tells them apart.
	var survivor database.Product
	if readErr := r.DB.Unscoped().First(&survivor, productID).Error; readErr != nil {
		if stderrors.Is(readErr, gorm.ErrRecordNotFound) {
			return true, true, nil
		}
		return false, true, readErr
	}
	return false, true, nil
}

func (r *ProductStockRepository) ConsumeProduct(productID, userID uint) error {
	product, err := getProductByID(r.DB, productID, userID)
	if err != nil {
		return err
	}
	product.RemovalReason = database.RemovalReasonConsumed
	// Save and delete must succeed together — a non-transactional failure in
	// between would leave an active product already marked as consumed.
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&product).Error; err != nil {
			return err
		}
		return tx.Delete(&database.Product{}, productID).Error
	})
}

// ConsumeProductPartial removes up to quantity packs and archives the row when
// the request covers its current stock. A zero or negative quantity consumes
// the whole stock, preserving the recipe-cook contract.
func (r *ProductStockRepository) ConsumeProductPartial(product *database.Product, quantity int, mutationID *string) (int, bool, bool, error) {
	return r.removeProductPartial(product, quantity, database.RemovalReasonConsumed, mutationID, "")
}

// WasteProductPartial is the waste counterpart of ConsumeProductPartial.
func (r *ProductStockRepository) WasteProductPartial(product *database.Product, quantity int, mutationID *string, cause string) (int, bool, bool, error) {
	return r.removeProductPartial(product, quantity, database.RemovalReasonWasted, mutationID, cause)
}

// removeProductPartial compares data values only (quantity, removal_reason and
// deleted_at), never updated_at, whose driver representation is not stable on
// legacy rows. removal_reason also prevents a second request from claiming an
// archived row. A mutation claim shares the transaction with the stock change,
// so an offline replay cannot apply it twice. The cause is why the packs were
// wasted; it is written beside the removal reason when the waste archives the
// row.
func (r *ProductStockRepository) removeProductPartial(product *database.Product, quantity int, reason string, mutationID *string, cause string) (removed int, fullyRemoved bool, applied bool, err error) {
	if mutationID != nil && strings.TrimSpace(*mutationID) == "" {
		mutationID = nil
	}

	if mutationID == nil && quantity > 0 && quantity < product.Quantity {
		result := r.DB.Model(&database.Product{}).
			Where("id = ?", product.ID).
			Where("household_id = ?", product.HouseholdID).
			Where(util.WhereDeletedIsNull).
			Where(util.WhereRemovalReasonUnclaimed).
			Where("quantity > ?", quantity).
			Update("quantity", gorm.Expr("quantity - ?", quantity))
		if result.Error != nil {
			return 0, false, false, result.Error
		}
		if result.RowsAffected > 0 {
			return quantity, false, true, nil
		}
	}

	var txErr error
	for range 2 {
		removed, fullyRemoved, applied = 0, false, false
		txErr = r.DB.Transaction(func(tx *gorm.DB) error {
			if mutationID != nil {
				claim := database.ProductMutationClaim{ProductID: product.ID, MutationID: *mutationID}
				claimResult := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
				if claimResult.Error != nil {
					return claimResult.Error
				}
				if claimResult.RowsAffected == 0 {
					var claimed database.ProductMutationClaim
					if err := tx.First(&claimed, "product_id = ? AND mutation_id = ?", product.ID, *mutationID).Error; err != nil {
						return errMutationClaimRefused
					}
					return errMutationAlreadyClaimed
				}
			}

			var current database.Product
			if getErr := tx.Where("id = ? AND household_id = ?", product.ID, product.HouseholdID).First(&current).Error; getErr != nil {
				return getErr
			}
			if quantity > 0 && quantity < current.Quantity {
				result := tx.Model(&database.Product{}).
					Where("id = ?", current.ID).
					Where("household_id = ?", current.HouseholdID).
					Where(util.WhereDeletedIsNull).
					Where(util.WhereRemovalReasonUnclaimed).
					Where("quantity > ?", quantity).
					Update("quantity", gorm.Expr("quantity - ?", quantity))
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return errors.ErrProductConcurrentModification
				}
				removed = quantity
				applied = true
				return nil
			}

			removed = current.Quantity
			fullyRemoved = true
			result := tx.Model(&database.Product{}).
				Where("id = ?", current.ID).
				Where("household_id = ?", current.HouseholdID).
				Where(util.WhereDeletedIsNull).
				Where(util.WhereRemovalReasonUnclaimed).
				Where("quantity = ?", current.Quantity).
				Updates(map[string]any{
					"removal_reason": reason,
					"waste_cause":    cause,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errors.ErrProductConcurrentModification
			}
			if err := tx.Delete(&database.Product{}, current.ID).Error; err != nil {
				return err
			}
			applied = true
			return nil
		})
		if txErr != errors.ErrProductConcurrentModification {
			break
		}
	}
	if stderrors.Is(txErr, errMutationAlreadyClaimed) {
		return 0, false, false, nil
	}
	if txErr != nil {
		return 0, false, false, txErr
	}
	return removed, fullyRemoved, applied, nil
}

func (r *ProductStockRepository) WasteProduct(productID, userID uint, cause string) error {
	product, err := getProductByID(r.DB, productID, userID)
	if err != nil {
		return err
	}
	// Single transaction: the removal-reason update and the soft-delete must
	// succeed together, or neither lands. The unclaimed-row precondition fails
	// the write when a concurrent request already claimed the row, instead of
	// silently overwriting its removal reason.
	return r.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&database.Product{}).
			Where("id = ?", product.ID).
			Where("household_id = ?", product.HouseholdID).
			Where(util.WhereDeletedIsNull).
			Where(util.WhereRemovalReasonUnclaimed).
			Updates(map[string]any{
				"removal_reason": database.RemovalReasonWasted,
				"waste_cause":    cause,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.ErrProductConcurrentModification
		}
		return tx.Delete(&database.Product{}, productID).Error
	})
}

func (r *ProductStockRepository) BulkConsumeProducts(productIDs []uint, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		if err := r.ConsumeProduct(productID, userID); err != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, err})
		}
	}
	return bulkErrors
}

func (r *ProductStockRepository) BulkWasteProducts(productIDs []uint, userID uint, cause string) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		if err := r.WasteProduct(productID, userID, cause); err != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, err})
		}
	}
	return bulkErrors
}
