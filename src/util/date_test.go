package util

import (
	"testing"
	"time"
)

// The 31st is the day that breaks the calendar arithmetic: adding or
// subtracting months from it overflows into the neighbouring month, so a
// bucket built with time.AddDate skips a month or names one twice.
func TestMonthShiftKeepsTheMonth(t *testing.T) {
	anchor := time.Date(2026, time.October, 31, 12, 0, 0, 0, time.Local)

	tests := []struct {
		months int
		want   string
	}{
		{months: 0, want: "2026-10"},
		{months: -1, want: "2026-09"},
		{months: -4, want: "2026-06"},
		{months: -5, want: "2026-05"},
		{months: 1, want: "2026-11"},
		{months: 2, want: "2026-12"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := MonthKey(MonthShift(anchor, tt.months)); got != tt.want {
				t.Errorf("MonthShift(anchor, %d) month = %q, want %q", tt.months, got, tt.want)
			}
		})
	}
}

func TestMonthShiftCrossesTheYear(t *testing.T) {
	anchor := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.Local)

	if got := MonthKey(MonthShift(anchor, -1)); got != "2025-12" {
		t.Errorf("MonthShift(anchor, -1) month = %q, want 2025-12", got)
	}
	if got := MonthKey(MonthShift(anchor, 12)); got != "2027-01" {
		t.Errorf("MonthShift(anchor, 12) month = %q, want 2027-01", got)
	}
}

func TestMonthStart(t *testing.T) {
	anchor := time.Date(2026, time.October, 31, 23, 59, 59, 0, time.Local)

	got := MonthStart(anchor)
	want := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("MonthStart = %v, want %v", got, want)
	}
}

func TestRecentMonthKeysAreConsecutive(t *testing.T) {
	anchor := time.Date(2026, time.October, 31, 0, 0, 0, 0, time.Local)

	got := RecentMonthKeys(anchor, 6)
	want := []string{"2026-05", "2026-06", "2026-07", "2026-08", "2026-09", "2026-10"}
	if len(got) != len(want) {
		t.Fatalf("RecentMonthKeys returned %d keys, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("RecentMonthKeys[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUpcomingMonthKeysAreConsecutive(t *testing.T) {
	anchor := time.Date(2026, time.October, 31, 0, 0, 0, 0, time.Local)

	got := UpcomingMonthKeys(anchor, 4)
	want := []string{"2026-10", "2026-11", "2026-12", "2027-01"}
	if len(got) != len(want) {
		t.Fatalf("UpcomingMonthKeys returned %d keys, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("UpcomingMonthKeys[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// A count that names no month yields no buckets rather than a panic.
func TestMonthKeysTolerateAnEmptyWindow(t *testing.T) {
	anchor := time.Date(2026, time.October, 31, 0, 0, 0, 0, time.Local)

	if got := RecentMonthKeys(anchor, 0); len(got) != 0 {
		t.Errorf("RecentMonthKeys(anchor, 0) = %v, want no keys", got)
	}
	if got := RecentMonthKeys(anchor, -3); len(got) != 0 {
		t.Errorf("RecentMonthKeys(anchor, -3) = %v, want no keys", got)
	}
	if got := UpcomingMonthKeys(anchor, 0); len(got) != 0 {
		t.Errorf("UpcomingMonthKeys(anchor, 0) = %v, want no keys", got)
	}
	if got := UpcomingMonthKeys(anchor, -3); len(got) != 0 {
		t.Errorf("UpcomingMonthKeys(anchor, -3) = %v, want no keys", got)
	}
}

// A window of no months has no start: the zero time leaves the window empty
// instead of covering the anchor's month behind the caller's back.
func TestEmptyWindowHasNoStart(t *testing.T) {
	anchor := time.Date(2026, time.October, 31, 0, 0, 0, 0, time.Local)

	if got := MonthWindowStart(anchor, 0); !got.IsZero() {
		t.Errorf("MonthWindowStart(anchor, 0) = %v, want the zero time", got)
	}
	if got := MonthWindowStart(anchor, -3); !got.IsZero() {
		t.Errorf("MonthWindowStart(anchor, -3) = %v, want the zero time", got)
	}
}

// The month helpers work in the container's zone: the same instant names the
// same month no matter which zone it comes in, so a row read back from the
// database and the window that filters it cannot land in different months.
func TestMonthHelpersUseTheContainerZone(t *testing.T) {
	plusTwo := time.FixedZone("CEST", 2*60*60)
	anchor := time.Date(2026, time.November, 1, 0, 30, 0, 0, plusTwo)

	if got := MonthKey(anchor); got != MonthKey(anchor.UTC()) {
		t.Errorf("MonthKey = %q and %q for the same instant, want one key",
			MonthKey(anchor), MonthKey(anchor.UTC()))
	}

	local := anchor.In(time.Local)
	monthStart := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.Local)
	if got := MonthStart(anchor); !got.Equal(monthStart) {
		t.Errorf("MonthStart = %v, want %v", got, monthStart)
	}

	wantKey := monthStart.Format(DefaultDateFormatMonthStr)
	if got := MonthKey(anchor); got != wantKey {
		t.Errorf("MonthKey = %q, want %q", got, wantKey)
	}

	recent := RecentMonthKeys(anchor, 2)
	wantRecent := []string{
		monthStart.AddDate(0, -1, 0).Format(DefaultDateFormatMonthStr),
		wantKey,
	}
	if len(recent) != len(wantRecent) {
		t.Fatalf("RecentMonthKeys returned %d keys, want %d", len(recent), len(wantRecent))
	}
	for i := range wantRecent {
		if recent[i] != wantRecent[i] {
			t.Errorf("RecentMonthKeys[%d] = %q, want %q", i, recent[i], wantRecent[i])
		}
	}

	upcoming := UpcomingMonthKeys(anchor, 2)
	wantUpcoming := []string{
		wantKey,
		monthStart.AddDate(0, 1, 0).Format(DefaultDateFormatMonthStr),
	}
	if len(upcoming) != len(wantUpcoming) {
		t.Fatalf("UpcomingMonthKeys returned %d keys, want %d", len(upcoming), len(wantUpcoming))
	}
	for i := range wantUpcoming {
		if upcoming[i] != wantUpcoming[i] {
			t.Errorf("UpcomingMonthKeys[%d] = %q, want %q", i, upcoming[i], wantUpcoming[i])
		}
	}
}

// The window start and the window keys describe the same window: the keys are
// the months that follow the start.
func TestMonthWindowStartOpensTheRecentMonthWindow(t *testing.T) {
	anchor := time.Date(2026, time.October, 31, 12, 0, 0, 0, time.Local)

	start := MonthWindowStart(anchor, 6)
	local := anchor.In(time.Local)
	want := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -5, 0)
	if got := MonthKey(start); got != MonthKey(want) {
		t.Errorf("MonthWindowStart(anchor, 6) month = %q, want %q", got, MonthKey(want))
	}
	if got := start.Day(); got != 1 {
		t.Errorf("MonthWindowStart day = %d, want 1", got)
	}

	keys := RecentMonthKeys(anchor, 6)
	if len(keys) == 0 {
		t.Fatal("RecentMonthKeys returned no keys")
	}
	if keys[0] != MonthKey(start) {
		t.Errorf("RecentMonthKeys[0] = %q, want the window start %q", keys[0], MonthKey(start))
	}
}
