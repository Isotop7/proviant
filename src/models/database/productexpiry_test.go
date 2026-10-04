package database

import (
	"testing"
	"time"
)

// The recipe suggestion cache key fingerprints this bucket, and the suggestion
// ranking scores by it. If the two ever disagreed the cache would serve a
// ranking its own inputs no longer produce, so the bucket is pinned here at its
// boundaries — including the case a calendar-date encoding got wrong: two
// timestamps on the same UTC date that straddle a bucket boundary.
func TestExpiryRankingDaysLeft(t *testing.T) {
	now := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	offset := func(days int) time.Time {
		return now.AddDate(0, 0, days)
	}
	// The next UTC day at a fixed wall-clock hour. The bucket boundary sits at
	// now+24h, which lands at 12:00 on that day, so hour 6 falls in the
	// preceding bucket and hour 18 in the following one — both on one UTC date.
	nextDay := now.AddDate(0, 0, 1)
	atHour := func(hour int) time.Time {
		return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), hour, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name     string
		expireAt time.Time
		want     int
	}{
		{"zero timestamp buckets to zero", time.Time{}, 0},
		{"already expired clamps to zero", now.Add(-24 * time.Hour), 0},
		{"just inside the next bucket stays zero", now.Add(23 * time.Hour), 0},
		{"exactly one bucket ahead", now.Add(24 * time.Hour), 1},
		{"mid bucket", offset(3), 3},
		{"horizon itself", offset(ExpiryRankingHorizonDays), ExpiryRankingHorizonDays},
		{"beyond the horizon clamps", offset(ExpiryRankingHorizonDays + 5), ExpiryRankingHorizonDays},
		{
			// The regression: same UTC date, different buckets. Encoding the
			// calendar date would give both of these the same key.
			name:     "same UTC date after the bucket boundary",
			expireAt: atHour(18),
			want:     1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ExpiryRankingDaysLeft(test.expireAt, now); got != test.want {
				t.Errorf("ExpiryRankingDaysLeft(%v, now) = %d, want %d", test.expireAt, got, test.want)
			}
		})
	}

	t.Run("same UTC date can hold two different buckets", func(t *testing.T) {
		early, late := atHour(6), atHour(18)
		if early.UTC().Format(time.DateOnly) != late.UTC().Format(time.DateOnly) {
			t.Fatalf("test setup broken: %v and %v are on different UTC dates", early, late)
		}
		if ExpiryRankingDaysLeft(early, now) == ExpiryRankingDaysLeft(late, now) {
			t.Error("two timestamps on one UTC date bucketed identically, so an expiry edit could change the ranking without changing the cache key")
		}
	})
}
