package database

import "time"

// ProductMutationClaim records one applied clientMutationId. The amount delta
// and its claim are written in one transaction, so a replayed delta loses the
// insert on the unique index and moves no amount.
//
// One row per mutation id rather than one token per product: a single slot only
// refuses a replay of the most recent mutation, and two drains of the same
// outbox apply their mutations in whatever order the network returns them.
//
// No gorm.Model on purpose: a soft delete would leave the row and its unique
// index entry in place, and the conflict path reads with Unscoped anyway, so a
// replayed mutation id would then report a delta as applied that never ran.
// Claims are removed only by the hard-deleting sweep in proviant.go.
type ProductMutationClaim struct {
	ID         uint      `gorm:"primaryKey" json:"-"`
	CreatedAt  time.Time `json:"-"`
	ProductID  uint      `gorm:"index;index:idx_product_mutation_claims_product_mutation,priority:1;not null" json:"-"`
	MutationID string    `gorm:"index:idx_product_mutation_claims_product_mutation,unique,priority:2;not null" json:"-"`
}
