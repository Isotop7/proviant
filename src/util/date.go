package util

import "time"

// The app has one calendar: the container's zone.
//
// Every timestamp the app stores is written in it — the SQLite driver keeps
// the offset it was given, MariaDB converts to the server's local time — and
// every window the app filters on is compared against those stored values. A
// bound that carries a different offset compares wall-clock strings against
// each other and loses the rows at the edge of the period, so the bounds and
// the rows have to speak the same calendar.
//
// The product labels are rendered in it as well (see models/api/product.go),
// so the month a product expires in is the month its label shows.

// MonthStart returns the first instant of the month t falls in, in the
// container's zone.
func MonthStart(t time.Time) time.Time {
	local := t.In(time.Local)
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.Local)
}

// MonthShift returns the first instant of the month that lies `months` away
// from the month t falls in. It differs from time.AddDate on purpose: AddDate
// carries the day of the month over and normalizes the overflow into the
// neighbouring month, so Oct 31 shifted by -4 months lands on Jul 1 instead of
// Jun 1. A month bucket carries no day, so dropping it keeps every shift
// inside the month it names.
func MonthShift(t time.Time, months int) time.Time {
	return MonthStart(t).AddDate(0, months, 0)
}

// MonthWindowStart returns the first instant of the month that opens the
// window of the `months` months ending with the month t falls in. It is the
// one place that decides how long a month window reaches back, so a caller
// that filters on a window and a caller that buckets one cannot disagree.
//
// A window of no months has no start: the zero time leaves the window empty
// instead of covering the anchor's month behind the caller's back.
func MonthWindowStart(t time.Time, months int) time.Time {
	if months <= 0 {
		return time.Time{}
	}
	return MonthShift(t, -(months - 1))
}

// MonthKey returns the key of the month t falls in ("2006-01"), in the
// container's zone.
func MonthKey(t time.Time) string {
	return t.In(time.Local).Format(DefaultDateFormatMonthStr)
}

// RecentMonthKeys returns the keys of the `months` months of the window that
// ends with the month `anchor` falls in, oldest first. A window of no months
// names no month.
func RecentMonthKeys(anchor time.Time, months int) []string {
	if months <= 0 {
		return nil
	}
	start := MonthWindowStart(anchor, months)
	keys := make([]string, months)
	for i := range months {
		keys[i] = MonthKey(start.AddDate(0, i, 0))
	}
	return keys
}

// UpcomingMonthKeys returns the keys of the `count` months that start with the
// month `anchor` falls in, oldest first. A count of no months names no month.
func UpcomingMonthKeys(anchor time.Time, count int) []string {
	if count <= 0 {
		return nil
	}
	keys := make([]string, count)
	for i := range count {
		keys[i] = MonthKey(MonthShift(anchor, i))
	}
	return keys
}
