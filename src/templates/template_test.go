package templates

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	dbModel "codeberg.org/isotop7/proviant/models/database"
)

func TestHumanDateFormatting(t *testing.T) {
	ts := time.Date(2026, 3, 7, 14, 5, 0, 0, time.UTC)

	if got := humanDate(ts); got != "07.03.2026" {
		t.Errorf("humanDate() = %q, want %q", got, "07.03.2026")
	}
	if got := humanDateTime(ts); got != "07.03.2026, 14:05" {
		t.Errorf("humanDateTime() = %q, want %q", got, "07.03.2026, 14:05")
	}
	if got := humanDateTimeFromSQL(gorm.DeletedAt{Time: ts, Valid: true}); got != "07.03.2026, 14:05" {
		t.Errorf("humanDateTimeFromSQL() = %q, want %q", got, "07.03.2026, 14:05")
	}
	if got := inputDate(ts); got != "2026-03-07" {
		t.Errorf("inputDate() = %q, want %q", got, "2026-03-07")
	}
	if got := today(); got != time.Now().Format("2006-01-02") {
		t.Errorf("today() = %q, want %q", got, time.Now().Format("2006-01-02"))
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04", now(), time.Local)
	if err != nil {
		t.Fatalf("now() = %q, not parseable as yyyy-mm-dd hh:mm", now())
	}
	if diff := time.Since(parsed); diff < -time.Minute || diff > time.Minute {
		t.Errorf("now() = %q, more than 1 minute from current time (%v)", now(), diff)
	}
}

func TestHasPassed(t *testing.T) {
	if !hasPassed(time.Now().Add(-time.Hour)) {
		t.Error("hasPassed(past) = false, want true")
	}
	if hasPassed(time.Now().Add(time.Hour)) {
		t.Error("hasPassed(future) = true, want false")
	}
}

func TestExpiryStatusClass(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "nodate"},
		{"expired", now.Add(-24 * time.Hour), "expired"},
		{"critical", now.Add(2 * 24 * time.Hour), "critical"},
		{"soon", now.Add(5 * 24 * time.Hour), "soon"},
		{"fresh", now.Add(30 * 24 * time.Hour), "fresh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expiryStatusClass(tt.t); got != tt.want {
				t.Errorf("expiryStatusClass() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpiryStatusIcon(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "dash-circle-fill"},
		{"expired", now.Add(-24 * time.Hour), "x-circle-fill"},
		{"critical", now.Add(2 * 24 * time.Hour), "exclamation-circle-fill"},
		{"soon", now.Add(5 * 24 * time.Hour), "clock-fill"},
		{"fresh", now.Add(30 * 24 * time.Hour), "check-circle-fill"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expiryStatusIcon(tt.t); got != tt.want {
				t.Errorf("expiryStatusIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpiryStatusLabel(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "No date"},
		{"expired", now.Add(-24 * time.Hour), "Expired"},
		{"critical", now.Add(2 * 24 * time.Hour), "Critical"},
		{"soon", now.Add(5 * 24 * time.Hour), "Expiring soon"},
		{"fresh", now.Add(30 * 24 * time.Hour), "Fresh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expiryStatusLabel(tt.t); got != tt.want {
				t.Errorf("expiryStatusLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpiryBadgeClass(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "bg-secondary"},
		{"expired", now.Add(-24 * time.Hour), "bg-danger"},
		{"critical", now.Add(2 * 24 * time.Hour), "bg-danger"},
		{"soon", now.Add(5 * 24 * time.Hour), "bg-warning"},
		{"fresh", now.Add(30 * 24 * time.Hour), "bg-success"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expiryBadgeClass(tt.t); got != tt.want {
				t.Errorf("expiryBadgeClass() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpiryTextClass(t *testing.T) {
	now := time.Now()
	if got := expiryTextClass(time.Time{}); got != "text-secondary" {
		t.Errorf("expiryTextClass(zero) = %q, want text-secondary", got)
	}
	if got := expiryTextClass(now.Add(2 * 24 * time.Hour)); got != "text-danger" {
		t.Errorf("expiryTextClass(critical) = %q, want text-danger", got)
	}
	if got := expiryTextClass(now.Add(5 * 24 * time.Hour)); got != "text-warning" {
		t.Errorf("expiryTextClass(soon) = %q, want text-warning", got)
	}
}

func TestExpiryColor(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "var(--fg-3)"},
		{"expired", now.Add(-24 * time.Hour), "var(--status-expired)"},
		{"critical", now.Add(2 * 24 * time.Hour), "var(--status-critical)"},
		{"soon", now.Add(5 * 24 * time.Hour), "var(--status-soon)"},
		{"fresh", now.Add(30 * 24 * time.Hour), "var(--fg-2)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expiryColor(tt.t); got != tt.want {
				t.Errorf("expiryColor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpiryUrgencyText(t *testing.T) {
	now := time.Now()
	if got := expiryUrgencyText(time.Time{}); got != "" {
		t.Errorf("urgency(zero) = %q, want empty", got)
	}

	// Expired 3 full days ago (plus a margin so integer day math is stable).
	expired := now.Add(-76 * time.Hour)
	if got := expiryUrgencyText(expired); got != "Expired 3 day(s) ago" {
		t.Errorf("urgency(expired 76h) = %q, want %q", got, "Expired 3 day(s) ago")
	}

	// Expires tomorrow: less than 24h away, at least a few hours.
	tomorrow := now.Add(20 * time.Hour)
	if got := expiryUrgencyText(tomorrow); got != "Expires tomorrow" {
		t.Errorf("urgency(in 20h) = %q, want %q", got, "Expires tomorrow")
	}

	// Within a week.
	fewDays := now.Add(3 * 24 * time.Hour)
	if got := expiryUrgencyText(fewDays); got != "Expires in 3 days" {
		t.Errorf("urgency(in 3d) = %q, want %q", got, "Expires in 3 days")
	}

	// Far away: no urgency text.
	if got := expiryUrgencyText(now.Add(60 * 24 * time.Hour)); got != "" {
		t.Errorf("urgency(in 60d) = %q, want empty", got)
	}
}

func TestExpiryDays(t *testing.T) {
	now := time.Now()
	if got := expiryDays(time.Time{}); got != 0 {
		t.Errorf("expiryDays(zero) = %d, want 0", got)
	}
	if got := expiryDays(now.Add(-76 * time.Hour)); got >= 0 {
		t.Errorf("expiryDays(expired) = %d, want negative", got)
	}
	if got := expiryDays(now.Add(3 * 24 * time.Hour)); got != 3 {
		t.Errorf("expiryDays(in 3d) = %d, want 3", got)
	}
}

func TestQueryWith(t *testing.T) {
	params := url.Values{}
	params.Set("status", "expired")

	got := string(queryWith(params, "page", "2"))
	if !strings.Contains(got, "status=expired") || !strings.Contains(got, "page=2") {
		t.Errorf("queryWith(add) = %q, want both params", got)
	}

	got = string(queryWith(params, "status", ""))
	if strings.Contains(got, "status") {
		t.Errorf("queryWith(empty value) = %q, want status removed", got)
	}
}

func TestStringSlice(t *testing.T) {
	got := stringSlice("a", "b")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("stringSlice() = %v, want [a b]", got)
	}
}

func TestBadgifyCategories(t *testing.T) {
	got := string(badgifyCategories("de:Deutschland, fr:Frankreich", 5))
	if !strings.Contains(got, `class="badge`) || !strings.Contains(got, "de") || !strings.Contains(got, "Deutschland") {
		t.Errorf("badgifyCategories() = %q, want badge markup", got)
	}

	limited := string(badgifyCategories("a,b,c", 2))
	if strings.Contains(limited, "c") {
		t.Errorf("badgifyCategories(limit 2) = %q, want third category dropped", limited)
	}

	plain := string(badgifyCategories("single-tag-without-colon", 5))
	if !strings.Contains(plain, "single-tag-without-colon") {
		t.Errorf("badgifyCategories(plain) = %q, want raw element", plain)
	}
}

func TestSplitString(t *testing.T) {
	got := string(splitString("a,b,c"))
	if !strings.Contains(got, "a<br>") || !strings.Contains(got, "b<br>") || !strings.Contains(got, "c<br>") {
		t.Errorf("splitString() = %q, want comma-split with <br>", got)
	}
}

func TestEmojifyFlag(t *testing.T) {
	if got := emojifyFlag("de:Name"); !strings.HasPrefix(got, "🇩🇪") || !strings.HasSuffix(got, ":Name<br>") {
		t.Errorf("emojifyFlag(de) = %q, want flag + name", got)
	}
	if got := emojifyFlag("nocolon"); got != "nocolon" {
		t.Errorf("emojifyFlag(nocolon) = %q, want unchanged", got)
	}
	if got := emojifyFlag("zz:Unknown"); !strings.HasPrefix(got, "zz:") {
		t.Errorf("emojifyFlag(unknown code) = %q, want code as fallback", got)
	}
	if got := emojifyFlag("DE:Uppercase"); !strings.HasPrefix(got, "🇩🇪") {
		t.Errorf("emojifyFlag(DE) = %q, want case-insensitive lookup", got)
	}
}

func TestFlagReplace(t *testing.T) {
	got := string(flagReplace("de:Deutsch,gb:UK"))
	if !strings.HasPrefix(got, "🇩🇪") || !strings.Contains(got, "🇬🇧") {
		t.Errorf("flagReplace() = %q, want both flags", got)
	}
}

func TestDerefUint(t *testing.T) {
	one := uint(1)
	if got := derefUint(&one); got != 1 {
		t.Errorf("derefUint(&1) = %d, want 1", got)
	}
	if got := derefUint(nil); got != 0 {
		t.Errorf("derefUint(nil) = %d, want 0", got)
	}
}

func TestEffectiveExpireAtTemplateFunc(t *testing.T) {
	expire := time.Now().Add(24 * time.Hour)
	p := &dbModel.Product{ExpireAt: expire}
	if got := effectiveExpireAt(p); !got.Equal(expire) {
		t.Errorf("effectiveExpireAt() = %v, want %v", got, expire)
	}
}

func TestSetExpiryThresholds(t *testing.T) {
	originalCritical, originalSoon := criticalThresholdDays, soonThresholdDays
	t.Cleanup(func() {
		criticalThresholdDays, soonThresholdDays = originalCritical, originalSoon
	})

	SetExpiryThresholds(1, 2)
	now := time.Now()
	if got := expiryStatusClass(now.Add(36 * time.Hour)); got != "soon" {
		t.Errorf("after SetExpiryThresholds(1,2): class = %q, want soon", got)
	}
}

func TestNewTemplateCache(t *testing.T) {
	cache, err := NewTemplateCache()
	if err != nil {
		t.Fatalf("NewTemplateCache() error = %v", err)
	}
	for _, want := range []string{"home.tmpl", "error.tmpl"} {
		if _, ok := cache[want]; !ok {
			t.Errorf("cache missing %q; have %d templates", want, len(cache))
		}
	}
}
