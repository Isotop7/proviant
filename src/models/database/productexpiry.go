package database

import "time"

// EffectiveExpireAt returns the earlier of the printed ExpireAt and
// OpenedAt + DaysAfterOpening days. Returns the zero time if neither is set.
//
// If OpenedAt is set but DaysAfterOpening is nil/<= 0, only ExpireAt is used.
// If DaysAfterOpening is set but OpenedAt is nil, only ExpireAt is used
// (a shelf-life-without-open-date is meaningless).
func (p *Product) EffectiveExpireAt() time.Time {
	if p == nil {
		return time.Time{}
	}
	return EffectiveExpireAt(p.ExpireAt, p.OpenedAt, p.DaysAfterOpening)
}

// EffectiveExpireAt applies the same precedence rule to the raw columns, for
// callers that hold only a narrow row projection rather than a Product.
//
// The rule lives here as a free function precisely so it cannot be copied.
// Recipe suggestion caching depends on both call sites agreeing exactly: the
// expiry-proximity ranking buckets a product through Product.EffectiveExpireAt
// while the cache-key fingerprint buckets the same product through a narrow
// row. A second hand-written copy of the precedence rule would silently
// desynchronize the cache key from the ranking the cached payload was ordered
// by, keeping the stale order alive for the whole TTL.
func EffectiveExpireAt(expireAt time.Time, openedAt *time.Time, daysAfterOpening *int) time.Time {
	var opened time.Time
	if openedAt != nil && daysAfterOpening != nil && *daysAfterOpening > 0 {
		opened = openedAt.AddDate(0, 0, *daysAfterOpening)
	}
	switch {
	case expireAt.IsZero() && opened.IsZero():
		return time.Time{}
	case expireAt.IsZero():
		return opened
	case opened.IsZero():
		return expireAt
	case opened.Before(expireAt):
		return opened
	default:
		return expireAt
	}
}

// ExpiryRankingHorizonDays is the width of the expiry-proximity ranking used by
// recipe suggestions. It doubles as the clamp on the day bucket, so a product
// further out than the horizon scores the same as one exactly on it.
const ExpiryRankingHorizonDays = 7

// ExpiryRankingDaysLeft buckets an expiry timestamp into the whole 24-hour
// spans between now and it, clamped to [0, ExpiryRankingHorizonDays]. A zero
// expireAt buckets to 0.
//
// It is the single source of truth for that bucket. Two call sites in different
// packages depend on it agreeing exactly: the recipe suggestion ranking scores a
// product by this bucket, and the suggestion cache key fingerprints it so an
// entry whose ranking no longer holds cannot be served. A second hand-written
// copy of the clamp would reintroduce exactly the drift this function exists to
// prevent — a cache key that moves when the ranking does not, or the reverse.
// now is passed in rather than read from the clock so one ranking pass and one
// fingerprint pass each judge every product against a single instant.
func ExpiryRankingDaysLeft(expireAt, now time.Time) int {
	if expireAt.IsZero() {
		return 0
	}
	daysLeft := int(expireAt.Sub(now).Hours() / 24)
	if daysLeft < 0 {
		return 0
	}
	if daysLeft > ExpiryRankingHorizonDays {
		return ExpiryRankingHorizonDays
	}
	return daysLeft
}

// ExpiryTrigger indicates which date fired the notification/status check.
type ExpiryTrigger string

const (
	ExpiryTriggerPrinted ExpiryTrigger = "printed"
	ExpiryTriggerOpened  ExpiryTrigger = "opened"
)

// TriggerForExpireAt returns which trigger (printed or opened) is the
// effective expiry for the product. Returns ExpiryTriggerOpened only when
// the opened-shelf-life date is strictly earlier than the printed date
// (or the printed date is zero). Useful for webhook payloads and analytics.
func (p *Product) TriggerForExpireAt() ExpiryTrigger {
	if p == nil {
		return ExpiryTriggerPrinted
	}
	if p.OpenedAt != nil && p.DaysAfterOpening != nil && *p.DaysAfterOpening > 0 {
		opened := p.OpenedAt.AddDate(0, 0, *p.DaysAfterOpening)
		if !opened.IsZero() && (p.ExpireAt.IsZero() || opened.Before(p.ExpireAt)) {
			return ExpiryTriggerOpened
		}
	}
	return ExpiryTriggerPrinted
}
