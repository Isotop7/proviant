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
	printed := p.ExpireAt
	var opened time.Time
	if p.OpenedAt != nil && p.DaysAfterOpening != nil && *p.DaysAfterOpening > 0 {
		opened = p.OpenedAt.AddDate(0, 0, *p.DaysAfterOpening)
	}
	switch {
	case printed.IsZero() && opened.IsZero():
		return time.Time{}
	case printed.IsZero():
		return opened
	case opened.IsZero():
		return printed
	case opened.Before(printed):
		return opened
	default:
		return printed
	}
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
