package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbController "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	gomail "gopkg.in/mail.v2"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// telegramRedirectTransport rewrites every request to the test server, so
// providers that hardcode https://api.telegram.org can be exercised offline.
type telegramRedirectTransport struct {
	target *url.URL
}

func (tr telegramRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = tr.target.Scheme
	req.URL.Host = tr.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func redirectClient(server *httptest.Server) *http.Client {
	target, _ := url.Parse(server.URL)
	return &http.Client{Transport: telegramRedirectTransport{target: target}, Timeout: 5 * time.Second}
}

// stubActivityLogRepo records Create calls for streak-reset assertions.
type stubActivityLogRepo struct {
	created []*dbModel.ActivityLog
}

func (s *stubActivityLogRepo) Create(_ context.Context, log *dbModel.ActivityLog) error {
	s.created = append(s.created, log)
	return nil
}
func (s *stubActivityLogRepo) GetByHousehold(context.Context, uint, int, int) ([]dbModel.ActivityLog, error) {
	return nil, nil
}
func (s *stubActivityLogRepo) GetByHouseholdCount(context.Context, uint) (int, error) {
	return len(s.created), nil
}

// vapidKeyRepo serves fixed VAPID keys for webpush tests.
type vapidKeyRepo struct {
	*repomocks.MockNotificationRepository
	publicKey  string
	privateKey string
}

func (m *vapidKeyRepo) GetVAPIDKeys() (string, string, error) { return m.publicKey, m.privateKey, nil }

// digestTargetRepo serves configurable mail digest targets.
type digestTargetRepo struct {
	*repomocks.MockNotificationRepository
	targets []models.HouseholdMailDigestTarget
}

func (m *digestTargetRepo) GetHouseholdsWithMailDigestEnabled() ([]models.HouseholdMailDigestTarget, error) {
	return m.targets, m.Err
}

// digestProductRepo serves a configurable mail digest product group.
type digestProductRepo struct {
	*repomocks.MockProductRepository
	group dbController.MailDigestProductGroup
	err   error
}

func (m *digestProductRepo) GetExpiringProductsForMailDigest(uint) (dbController.MailDigestProductGroup, error) {
	return m.group, m.err
}

// wasteStatsErrRepo fails only the waste stats lookup.
type wasteStatsErrRepo struct {
	*repomocks.MockNotificationRepository
}

func (m *wasteStatsErrRepo) GetWasteStatsForHousehold(uint, time.Time) (models.WasteStats, error) {
	return models.WasteStats{}, errors.New("stats unavailable")
}

// telegramLinkErrRepo fails only the chat ID update.
type telegramLinkErrRepo struct {
	*repomocks.MockNotificationRepository
}

func (m *telegramLinkErrRepo) SetTelegramChatID(uint, string) error { return errors.New("db down") }

// streakUpdateErrRepo fails only the streak write.
type streakUpdateErrRepo struct {
	*repomocks.MockStreakRepository
}

func (m *streakUpdateErrRepo) UpdateStreak(*dbModel.WasteStreak) error {
	return errors.New("write failed")
}

// unknownProvider exercises the default branch of sendViaProvider.
type unknownProvider struct{ sent bool }

func (p *unknownProvider) GetProviderType() string { return "webpush" }
func (p *unknownProvider) IsConfigured() bool      { return true }
func (p *unknownProvider) SendNotification(*dbModel.Product, any) error {
	p.sent = true
	return nil
}

// invitationMarkRecorder records which invitation-mark calls the controller
// makes while delegating everything else to the mock repository.
type invitationMarkRecorder struct {
	*repomocks.MockNotificationRepository
	markSent   []uint
	markFailed []uint
}

func (r *invitationMarkRecorder) MarkInvitationSent(invitationID uint) error {
	r.markSent = append(r.markSent, invitationID)
	return r.MockNotificationRepository.MarkInvitationSent(invitationID)
}

func (r *invitationMarkRecorder) MarkInvitationSendFailed(invitationID uint) error {
	r.markFailed = append(r.markFailed, invitationID)
	return r.MockNotificationRepository.MarkInvitationSendFailed(invitationID)
}

// syncLogBuffer is a goroutine-safe sink for log lines written by scheduler
// goroutines, so tests can assert on what they log before sleeping.
type syncLogBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncLogBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncLogBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// waitForLog polls until substr appears in buf or roughly 2s elapse.
func waitForLog(buf *syncLogBuffer, substr string) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), substr) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return strings.Contains(buf.String(), substr)
}

// scheduledRunTime extracts the RFC3339 timestamp a scheduler logged after
// its "next run at " marker.
func scheduledRunTime(t *testing.T, logOut string) time.Time {
	t.Helper()
	const marker = "next run at "
	idx := strings.Index(logOut, marker)
	if idx < 0 {
		t.Fatalf("log contains no %q entry: %s", marker, logOut)
	}
	rest := logOut[idx+len(marker):]
	end := strings.IndexAny(rest, "\"\r\n")
	if end < 0 {
		t.Fatalf("unterminated next-run entry in log: %s", logOut)
	}
	next, err := time.Parse(time.RFC3339, rest[:end])
	if err != nil {
		t.Fatalf("unparseable next-run time %q: %v", rest[:end], err)
	}
	return next
}

func newExtraNotificationController(mockRepos *repomocks.MockRepositoryContainer, config *configuration.NotificationConfiguration) *NotificationController {
	logger := zerolog.Nop()
	return &NotificationController{
		Logger:           &logger,
		Configuration:    config,
		NotificationRepo: mockRepos.Notifications,
		ProductRepo:      mockRepos.Products,
	}
}

func TestSetActivityLogRepo(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
	stub := &stubActivityLogRepo{}
	nc.SetActivityLogRepo(stub)
	if nc.ActivityLogRepo != stub {
		t.Error("SetActivityLogRepo did not attach the repository")
	}
}

func TestNewWebPushProvider(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
	provider := nc.newWebPushProvider()
	if provider == nil {
		t.Fatal("newWebPushProvider returned nil")
	}
	if provider.NotificationRepo != nc.NotificationRepo {
		t.Error("webpush provider does not carry the notification repository")
	}
}

func TestGetUserTelegramBotUsernameStored(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})

	nc.botUsernames.Store(uint(5), "testbot")
	if got := nc.GetUserTelegramBotUsername(5); got != "testbot" {
		t.Errorf("GetUserTelegramBotUsername(5) = %q, want %q", got, "testbot")
	}
	nc.botUsernames.Store(uint(6), 42)
	if got := nc.GetUserTelegramBotUsername(6); got != "" {
		t.Errorf("GetUserTelegramBotUsername(6) = %q, want empty for non-string entry", got)
	}
}

func TestSendPasswordReset(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	smtpConfig := &configuration.NotificationConfiguration{SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}}

	t.Run("not configured returns error", func(t *testing.T) {
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		err := nc.SendPasswordReset("test@example.com", "user1", "token", "http://example.com", time.Now())
		if err == nil || err.Error() != MsgEmailProviderNotConfigured {
			t.Errorf("err = %v, want %q", err, MsgEmailProviderNotConfigured)
		}
	})

	t.Run("success", func(t *testing.T) {
		nc := newExtraNotificationController(mockRepos, smtpConfig)
		sent := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			sent++
			if to := m.GetHeader("To"); len(to) != 1 || to[0] != "test@example.com" {
				t.Errorf("To header = %v, want [test@example.com]", to)
			}
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		if err := nc.SendPasswordReset("test@example.com", "user1", "token", "http://example.com", time.Now()); err != nil {
			t.Errorf("SendPasswordReset() error = %v", err)
		}
		if sent != 1 {
			t.Errorf("emailSendFunc called %d times, want 1", sent)
		}
	})

	t.Run("send failure returns error", func(t *testing.T) {
		nc := newExtraNotificationController(mockRepos, smtpConfig)
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error { return fmt.Errorf("smtp down") }
		defer func() { emailSendFunc = origFunc }()

		if err := nc.SendPasswordReset("test@example.com", "user1", "token", "http://example.com", time.Now()); err == nil {
			t.Error("SendPasswordReset() error = nil, want send failure")
		}
	})
}

func TestProcessMonthlyWasteReports(t *testing.T) {
	smtpConfig := &configuration.NotificationConfiguration{SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}}

	t.Run("households fetch error", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.Err = gorm.ErrInvalidData
		mockRepos.Notifications.HouseholdTargets = []models.HouseholdReportTarget{{
			HouseholdID: 3, HouseholdName: "Home", Recipients: []string{"a@example.com"},
		}}
		nc := newExtraNotificationController(mockRepos, smtpConfig)

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processMonthlyWasteReports(nc.newEmailProvider())
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (fetch error aborts the run)", emails)
		}
	})

	t.Run("stats error skips household", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.HouseholdTargets = []models.HouseholdReportTarget{{
			HouseholdID: 3, HouseholdName: "Home", Recipients: []string{"a@example.com"},
		}}
		nc := newExtraNotificationController(mockRepos, smtpConfig)
		nc.NotificationRepo = &wasteStatsErrRepo{mockRepos.Notifications}

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processMonthlyWasteReports(nc.newEmailProvider())
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (household with a stats error is skipped)", emails)
		}
	})

	t.Run("email and telegram fan-out", func(t *testing.T) {
		var telegramHits int32
		telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&telegramHits, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer telegramServer.Close()

		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.HouseholdTargets = []models.HouseholdReportTarget{{
			HouseholdID:        3,
			HouseholdName:      "Home",
			Recipients:         []string{"a@example.com"},
			TelegramRecipients: []models.TelegramRecipient{{ChatID: "55", BotToken: "BT"}},
		}}
		mockRepos.Notifications.WasteStats = models.WasteStats{MonthLabel: "September 2026", WastedCount: 2}
		nc := newExtraNotificationController(mockRepos, smtpConfig)
		nc.telegramClient = redirectClient(telegramServer)

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processMonthlyWasteReports(nc.newEmailProvider())
		if emails != 1 {
			t.Errorf("emails sent = %d, want 1", emails)
		}
		if got := atomic.LoadInt32(&telegramHits); got != 1 {
			t.Errorf("telegram hits = %d, want 1", got)
		}
	})
}

func TestSendMonthlyWasteReportToEmailRecipients(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	stats := &models.WasteStats{MonthLabel: "September 2026", HouseholdName: "Home"}

	t.Run("provider not configured sends nothing", func(t *testing.T) {
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		provider := &EmailNotificationProvider{}
		nc.sendMonthlyWasteReportToEmailRecipients(provider, []string{"a@example.com"}, stats, 1)
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (unconfigured provider must not send)", emails)
		}
	})

	t.Run("send failure is logged not fatal", func(t *testing.T) {
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return errors.New("smtp down")
		}
		defer func() { emailSendFunc = origFunc }()

		provider := &EmailNotificationProvider{Configuration: configuration.SMTPConfiguration{Host: "h", Port: 587}}
		nc.sendMonthlyWasteReportToEmailRecipients(provider, []string{"a@example.com"}, stats, 1)
		if emails != 1 {
			t.Errorf("send attempts = %d, want 1 (failure is logged, not fatal)", emails)
		}
	})
}

func TestSendMonthlyWasteReportToTelegramRecipients(t *testing.T) {
	stats := &models.WasteStats{MonthLabel: "September 2026", HouseholdName: "Home"}

	t.Run("success and failure", func(t *testing.T) {
		var hits int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/botOK/sendMessage" {
				atomic.AddInt32(&hits, 1)
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.telegramClient = redirectClient(server)

		recipients := []models.TelegramRecipient{{ChatID: "1", BotToken: "OK"}, {ChatID: "2", BotToken: "BAD"}}
		nc.sendMonthlyWasteReportToTelegramRecipients(recipients, stats, 9)
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Errorf("telegram hits = %d, want 1 (only the OK bot succeeds)", got)
		}
	})
}

func TestDispatchMonthlyWasteReportsStarts(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	buf := &syncLogBuffer{}
	logger := zerolog.New(buf)
	nc := &NotificationController{
		Logger: &logger,
		Configuration: &configuration.NotificationConfiguration{
			MonthlyWasteReport: configuration.MonthlyWasteReportConfiguration{Day: 1, Hour: 0},
		},
		NotificationRepo: mockRepos.Notifications,
		ProductRepo:      mockRepos.Products,
	}

	// The goroutine sleeps until the configured day of the next month, so its
	// only observable effect is the next-run time it logs before sleeping.
	nc.DispatchMonthlyWasteReports()
	if !waitForLog(buf, "Monthly waste report: next run at") {
		t.Fatal("monthly waste report goroutine never logged its next run time")
	}
	next := scheduledRunTime(t, buf.String())
	now := time.Now()
	if !next.After(now) {
		t.Errorf("next run = %v, want a time after %v", next, now)
	}
	if next.After(now.AddDate(0, 2, 0)) {
		t.Errorf("next run = %v, want it within two months of %v", next, now)
	}
}

func TestDispatchStreakUpdatesStarts(t *testing.T) {
	t.Run("schedules the next midnight UTC run", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		buf := &syncLogBuffer{}
		logger := zerolog.New(buf)
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
			StreakRepo:       mockRepos.Streaks,
		}

		// The goroutine sleeps until the next midnight UTC, so its only
		// observable effect is the next-run time it logs before sleeping.
		nc.DispatchStreakUpdates()
		if !waitForLog(buf, "Streak updater: next run at") {
			t.Fatal("streak updater goroutine never logged its next run time")
		}
		next := scheduledRunTime(t, buf.String())
		now := time.Now()
		if !next.After(now) {
			t.Errorf("next run = %v, want a time after %v", next, now)
		}
		if next.After(now.Add(25 * time.Hour)) {
			t.Errorf("next run = %v, want it within 25h of %v", next, now)
		}
	})

	t.Run("nil streak repo starts no goroutine", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		buf := &syncLogBuffer{}
		logger := zerolog.New(buf)
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}

		nc.DispatchStreakUpdates()
		if !strings.Contains(buf.String(), "StreakRepo not set") {
			t.Errorf("log = %q, want the skipped-scheduler warning", buf.String())
		}
		if strings.Contains(buf.String(), "next run at") {
			t.Error("streak scheduler goroutine started despite a nil StreakRepo")
		}
	})
}

func TestProcessStreakUpdates(t *testing.T) {
	t.Run("fetch error returns", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Streaks.Streaks = []dbModel.WasteStreak{{
			HouseholdID: 1, CurrentStreak: 5, LongestStreak: 5, LastCheckedDate: time.Now().UTC().AddDate(0, 0, -1),
		}}
		mockRepos.Streaks.Err = gorm.ErrInvalidData
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.StreakRepo = mockRepos.Streaks
		nc.processStreakUpdates()
		if got := mockRepos.Streaks.Streaks[0].CurrentStreak; got != 5 {
			t.Errorf("CurrentStreak = %d, want 5 (fetch error must skip processing)", got)
		}
	})

	t.Run("increment, milestone and reset", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		yesterday := time.Now().UTC().AddDate(0, 0, -1)
		now := time.Now().UTC()
		wasted := now
		mockRepos.Streaks.Streaks = []dbModel.WasteStreak{
			{Model: gorm.Model{ID: 1}, HouseholdID: 1, CurrentStreak: 6, LongestStreak: 6, LastCheckedDate: yesterday},
			{Model: gorm.Model{ID: 2}, HouseholdID: 2, CurrentStreak: 3, LastCheckedDate: yesterday, LastWastedDate: &wasted},
		}
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.StreakRepo = mockRepos.Streaks
		activityRepo := &stubActivityLogRepo{}
		nc.ActivityLogRepo = activityRepo

		nc.processStreakUpdates()

		streaks := mockRepos.Streaks.Streaks
		if streaks[0].CurrentStreak != 7 || streaks[0].LongestStreak != 7 {
			t.Errorf("streak[0] = %d/%d, want 7/7 (incremented, new longest)", streaks[0].CurrentStreak, streaks[0].LongestStreak)
		}
		if streaks[1].CurrentStreak != 0 || streaks[1].LastWastedDate != nil {
			t.Errorf("streak[1] = %d (wasted %v), want 0/nil after reset", streaks[1].CurrentStreak, streaks[1].LastWastedDate)
		}
		if len(activityRepo.created) != 1 {
			t.Fatalf("activity logs = %d, want 1 (streak reset recorded)", len(activityRepo.created))
		}
		log := activityRepo.created[0]
		if log.Action != dbModel.ActivityActionStreakReset || log.Quantity != 3 || log.HouseholdID != 2 {
			t.Errorf("activity log = %+v, want streak_reset/3/household 2", log)
		}
		if log.ProductName != "Streak of 3 days lost" {
			t.Errorf("ProductName = %q, want %q", log.ProductName, "Streak of 3 days lost")
		}
	})

	t.Run("update error is logged not fatal", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		yesterday := time.Now().UTC().AddDate(0, 0, -1)
		mockRepos.Streaks.Streaks = []dbModel.WasteStreak{
			{HouseholdID: 1, LastCheckedDate: yesterday},
			{HouseholdID: 2, LastCheckedDate: yesterday},
		}
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.StreakRepo = &streakUpdateErrRepo{mockRepos.Streaks}
		nc.processStreakUpdates()
		streaks := mockRepos.Streaks.Streaks
		if streaks[0].CurrentStreak != 1 || streaks[1].CurrentStreak != 1 {
			t.Errorf("streaks = %d/%d, want 1/1 (update errors must not abort the loop)",
				streaks[0].CurrentStreak, streaks[1].CurrentStreak)
		}
	})
}

func TestProcessHouseholdStreak(t *testing.T) {
	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)

	newNC := func(mockRepos *repomocks.MockRepositoryContainer) (*NotificationController, *stubActivityLogRepo) {
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		activityRepo := &stubActivityLogRepo{}
		nc.ActivityLogRepo = activityRepo
		return nc, activityRepo
	}

	t.Run("increment keeps longest", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc, activityRepo := newNC(mockRepos)
		streak := &dbModel.WasteStreak{HouseholdID: 1, CurrentStreak: 2, LongestStreak: 5, LastCheckedDate: yesterday}
		nc.processHouseholdStreak(streak, now, []int{7})
		if streak.CurrentStreak != 3 || streak.LongestStreak != 5 {
			t.Errorf("streak = %d/%d, want 3/5", streak.CurrentStreak, streak.LongestStreak)
		}
		if !streak.LastCheckedDate.Equal(now) {
			t.Errorf("LastCheckedDate = %v, want %v", streak.LastCheckedDate, now)
		}
		if len(activityRepo.created) != 0 {
			t.Errorf("activity logs = %d, want 0", len(activityRepo.created))
		}
	})

	t.Run("milestone fires at exact streak", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{} // fan-out loops over zero recipients
		nc, _ := newNC(mockRepos)
		streak := &dbModel.WasteStreak{HouseholdID: 1, CurrentStreak: 6, LongestStreak: 6, LastCheckedDate: yesterday}
		nc.processHouseholdStreak(streak, now, []int{7})
		if streak.CurrentStreak != 7 {
			t.Errorf("CurrentStreak = %d, want 7", streak.CurrentStreak)
		}
	})

	t.Run("reset with previous streak notifies", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc, activityRepo := newNC(mockRepos)
		wasted := now
		streak := &dbModel.WasteStreak{HouseholdID: 4, CurrentStreak: 4, LastCheckedDate: yesterday, LastWastedDate: &wasted}
		nc.processHouseholdStreak(streak, now, []int{7})
		if streak.CurrentStreak != 0 || streak.LastWastedDate != nil {
			t.Errorf("streak = %d (wasted %v), want reset", streak.CurrentStreak, streak.LastWastedDate)
		}
		if len(activityRepo.created) != 1 {
			t.Fatalf("activity logs = %d, want 1", len(activityRepo.created))
		}
	})

	t.Run("reset without previous streak stays silent", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc, activityRepo := newNC(mockRepos)
		wasted := now
		streak := &dbModel.WasteStreak{HouseholdID: 4, CurrentStreak: 0, LastCheckedDate: yesterday, LastWastedDate: &wasted}
		nc.processHouseholdStreak(streak, now, []int{7})
		if streak.CurrentStreak != 0 {
			t.Errorf("CurrentStreak = %d, want 0", streak.CurrentStreak)
		}
		if len(activityRepo.created) != 0 {
			t.Errorf("activity logs = %d, want 0 (no streak to lose)", len(activityRepo.created))
		}
	})
}

func TestPluralS(t *testing.T) {
	if got := pluralS(1); got != "" {
		t.Errorf("pluralS(1) = %q, want empty", got)
	}
	if got := pluralS(2); got != "s" {
		t.Errorf("pluralS(2) = %q, want %q", got, "s")
	}
	if got := pluralS(0); got != "s" {
		t.Errorf("pluralS(0) = %q, want %q", got, "s")
	}
}

func TestSendStreakResetNotifications(t *testing.T) {
	smtpNtfyConfig := &configuration.NotificationConfiguration{
		SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587},
		Ntfy: configuration.NtfyConfiguration{URL: "", Topic: "proviant"},
	}

	t.Run("preferences error returns", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{
			{EmailEnabled: true, EmailAddress: "a@example.com"},
		}
		mockRepos.Notifications.Err = gorm.ErrInvalidData
		nc := newExtraNotificationController(mockRepos, smtpNtfyConfig)

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.sendStreakResetNotifications(1, 5)
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (preferences fetch error aborts the fan-out)", emails)
		}
	})

	t.Run("email, ntfy and telegram fan-out", func(t *testing.T) {
		var ntfyHits, telegramHits int32
		ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&ntfyHits, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer ntfyServer.Close()
		telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&telegramHits, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer telegramServer.Close()

		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{
			{EmailEnabled: true, EmailAddress: "a@example.com"},
			{NtfyEnabled: true, NtfyURL: ntfyServer.URL, NtfyTopic: "proviant"},
			{TelegramEnabled: true, TelegramChatID: "77", TelegramBotToken: "BT"},
		}
		config := *smtpNtfyConfig
		config.Ntfy.URL = ntfyServer.URL
		nc := newExtraNotificationController(mockRepos, &config)
		nc.telegramClient = redirectClient(telegramServer)

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.sendStreakResetNotifications(2, 5)
		if emails != 1 {
			t.Errorf("emails = %d, want 1", emails)
		}
		if got := atomic.LoadInt32(&ntfyHits); got != 1 {
			t.Errorf("ntfy hits = %d, want 1", got)
		}
		if got := atomic.LoadInt32(&telegramHits); got != 1 {
			t.Errorf("telegram hits = %d, want 1", got)
		}
	})
}

func TestSendStreakResetEmailIfEnabled(t *testing.T) {
	configured := &EmailNotificationProvider{Configuration: configuration.SMTPConfiguration{Host: "h", Port: 587}}

	t.Run("recipient disabled sends nothing", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.sendStreakResetEmailIfEnabled(3, &models.NotificationRecipientInfo{}, configured)
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (disabled recipient must not send)", emails)
		}
	})

	t.Run("provider not configured sends nothing", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		pref := &models.NotificationRecipientInfo{EmailEnabled: true, EmailAddress: "a@example.com"}
		nc.sendStreakResetEmailIfEnabled(3, pref, &EmailNotificationProvider{})
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (unconfigured provider must not send)", emails)
		}
	})

	t.Run("enabled sends", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		pref := &models.NotificationRecipientInfo{EmailEnabled: true, EmailAddress: "a@example.com"}
		nc.sendStreakResetEmailIfEnabled(3, pref, configured)
		if emails != 1 {
			t.Errorf("emails = %d, want 1", emails)
		}
	})

	t.Run("send failure is logged", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return errors.New("smtp down")
		}
		defer func() { emailSendFunc = origFunc }()

		pref := &models.NotificationRecipientInfo{EmailEnabled: true, EmailAddress: "a@example.com"}
		nc.sendStreakResetEmailIfEnabled(3, pref, configured)
		if emails != 1 {
			t.Errorf("send attempts = %d, want 1 (failure is logged, not fatal)", emails)
		}
	})
}

func TestSendStreakResetNtfyIfEnabled(t *testing.T) {
	newNtfy := func(server *httptest.Server) *NtfyNotificationProvider {
		logger := zerolog.Nop()
		return &NtfyNotificationProvider{
			Configuration: configuration.NtfyConfiguration{URL: server.URL, Topic: "proviant"},
			Logger:        &logger,
			HTTPClient:    server.Client(),
		}
	}

	t.Run("enabled sends", func(t *testing.T) {
		var hits int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		pref := &models.NotificationRecipientInfo{NtfyEnabled: true}
		nc.sendStreakResetNtfyIfEnabled(3, pref, newNtfy(server))
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Errorf("ntfy hits = %d, want 1", got)
		}
	})

	t.Run("disabled sends nothing", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("ntfy server must not be called for a disabled recipient")
		}))
		defer server.Close()

		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.sendStreakResetNtfyIfEnabled(3, &models.NotificationRecipientInfo{}, newNtfy(server))
	})
}

func TestSendStreakResetTelegramIfEnabled(t *testing.T) {
	t.Run("enabled sends", func(t *testing.T) {
		var hits int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.telegramClient = redirectClient(server)
		pref := &models.NotificationRecipientInfo{TelegramEnabled: true, TelegramChatID: "77", TelegramBotToken: "BT"}
		nc.sendStreakResetTelegramIfEnabled(3, pref)
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Errorf("telegram hits = %d, want 1", got)
		}
	})

	t.Run("disabled sends nothing", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("telegram server must not be called for a disabled recipient")
		}))
		defer server.Close()

		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.telegramClient = redirectClient(server)
		nc.sendStreakResetTelegramIfEnabled(3, &models.NotificationRecipientInfo{})
	})
}

func TestSendStreakNtfyIfEnabled(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	provider := &NtfyNotificationProvider{
		Configuration: configuration.NtfyConfiguration{URL: server.URL, Topic: "proviant"},
		Logger:        &logger,
		HTTPClient:    server.Client(),
	}
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
	pref := &models.NotificationRecipientInfo{NtfyEnabled: true}
	nc.sendStreakNtfyIfEnabled(7, pref, provider)
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("ntfy hits = %d, want 1", got)
	}
}

func TestStartMailDigestSchedulerDisabled(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{
		MailDigest: configuration.MailDigestConfiguration{Enabled: false},
	})
	nc.StartMailDigestScheduler("http://example.com")
}

func TestProcessMailDigestEmails(t *testing.T) {
	smtpConfig := &configuration.NotificationConfiguration{SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}}
	dueUser := models.MailDigestUser{UserID: 1, Email: "a@example.com", MailDigestFrequency: authentication.MailDigestFrequencyDaily, MailDigestToken: "tok123"}

	t.Run("targets fetch error", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		repo := &digestTargetRepo{MockNotificationRepository: mockRepos.Notifications}
		repo.targets = []models.HouseholdMailDigestTarget{{
			HouseholdID: 1, HouseholdName: "Home", Users: []models.MailDigestUser{dueUser},
		}}
		mockRepos.Notifications.Err = gorm.ErrInvalidData
		nc := newExtraNotificationController(mockRepos, smtpConfig)
		nc.NotificationRepo = repo
		nc.ProductRepo = &digestProductRepo{
			MockProductRepository: mockRepos.Products,
			group:                 dbController.MailDigestProductGroup{Today: []dbModel.Product{{ProductName: "Milk"}}},
		}

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processMailDigestEmails("http://example.com", time.Now().Format("15:04"))
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (targets fetch error aborts the run)", emails)
		}
	})

	t.Run("user not due is skipped", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		repo := &digestTargetRepo{MockNotificationRepository: mockRepos.Notifications}
		repo.targets = []models.HouseholdMailDigestTarget{{
			HouseholdID: 1, HouseholdName: "Home",
			Users: []models.MailDigestUser{{UserID: 1, Email: "a@example.com", MailDigestFrequency: authentication.MailDigestFrequencyDisabled}},
		}}
		nc := newExtraNotificationController(mockRepos, smtpConfig)
		nc.NotificationRepo = repo
		// Non-empty groups prove the frequency check is the only reason no email goes out.
		nc.ProductRepo = &digestProductRepo{
			MockProductRepository: mockRepos.Products,
			group:                 dbController.MailDigestProductGroup{Today: []dbModel.Product{{ProductName: "Milk"}}},
		}

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processMailDigestEmails("http://example.com", "08:00")
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (a user not due is skipped)", emails)
		}
	})

	t.Run("products error continues", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		repo := &digestTargetRepo{MockNotificationRepository: mockRepos.Notifications}
		repo.targets = []models.HouseholdMailDigestTarget{{HouseholdID: 1, HouseholdName: "Home", Users: []models.MailDigestUser{dueUser}}}
		nc := newExtraNotificationController(mockRepos, smtpConfig)
		nc.NotificationRepo = repo
		nc.ProductRepo = &digestProductRepo{
			MockProductRepository: mockRepos.Products,
			group:                 dbController.MailDigestProductGroup{Today: []dbModel.Product{{ProductName: "Milk"}}},
			err:                   errors.New("db down"),
		}

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processMailDigestEmails("http://example.com", time.Now().Format("15:04"))
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (products fetch error skips the user)", emails)
		}
	})

	t.Run("empty groups skip the email", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		repo := &digestTargetRepo{MockNotificationRepository: mockRepos.Notifications}
		repo.targets = []models.HouseholdMailDigestTarget{{HouseholdID: 1, HouseholdName: "Home", Users: []models.MailDigestUser{dueUser}}}
		nc := newExtraNotificationController(mockRepos, smtpConfig)
		nc.NotificationRepo = repo
		nc.ProductRepo = &digestProductRepo{MockProductRepository: mockRepos.Products}

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processMailDigestEmails("http://example.com", time.Now().Format("15:04"))
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (empty product groups skip the email)", emails)
		}
	})

	t.Run("digest email sent", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		repo := &digestTargetRepo{MockNotificationRepository: mockRepos.Notifications}
		repo.targets = []models.HouseholdMailDigestTarget{{HouseholdID: 1, HouseholdName: "Home", Users: []models.MailDigestUser{dueUser}}}
		nc := newExtraNotificationController(mockRepos, smtpConfig)
		nc.NotificationRepo = repo
		nc.ProductRepo = &digestProductRepo{
			MockProductRepository: mockRepos.Products,
			group:                 dbController.MailDigestProductGroup{Today: []dbModel.Product{{ProductName: "Milk"}}},
		}

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			if to := m.GetHeader("To"); len(to) != 1 || to[0] != "a@example.com" {
				t.Errorf("To header = %v, want [a@example.com]", to)
			}
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		// shouldSendDigestNow reads its own time.Now(); a minute tick between
		// computing defaultTime and the call would make the user look not-due.
		// Retry with a freshly recomputed defaultTime each attempt.
		sent := 0
		for attempt := 0; attempt < 3 && sent == 0; attempt++ {
			emails = 0
			nc.processMailDigestEmails("http://example.com", time.Now().Format("15:04"))
			if emails == 1 {
				sent = 1
			}
		}
		if sent != 1 {
			t.Errorf("emails = %d, want 1 (after up to 3 attempts)", emails)
		}
	})
}

func TestShouldSendDigestNow(t *testing.T) {
	monday := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)  // 2026-10-05 is a Monday
	tuesday := time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC) // Tuesday
	tests := []struct {
		name        string
		frequency   string
		defaultTime string
		now         time.Time
		want        bool
	}{
		{name: "daily at target time", frequency: authentication.MailDigestFrequencyDaily, defaultTime: "08:00", now: monday, want: true},
		{name: "daily off target time", frequency: authentication.MailDigestFrequencyDaily, defaultTime: "08:00", now: monday.Add(time.Minute), want: false},
		{name: "weekly on Monday at target time", frequency: authentication.MailDigestFrequencyWeekly, defaultTime: "08:00", now: monday, want: true},
		{name: "weekly on Tuesday skipped", frequency: authentication.MailDigestFrequencyWeekly, defaultTime: "08:00", now: tuesday, want: false},
		{name: "unknown frequency never sends", frequency: authentication.MailDigestFrequencyDisabled, defaultTime: "08:00", now: monday, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepos := repomocks.NewMockRepositoryContainer()
			nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
			if got := nc.shouldSendDigestNow(tt.frequency, tt.defaultTime, tt.now); got != tt.want {
				t.Errorf("shouldSendDigestNow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input   string
		wantH   int
		wantMin int
	}{
		{input: "08:30", wantH: 8, wantMin: 30},
		{input: "23:59", wantH: 23, wantMin: 59},
		{input: "garbage", wantH: 8, wantMin: 0},
		{input: "8", wantH: 8, wantMin: 0},
		{input: "", wantH: 8, wantMin: 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			h, m, s := parseTime(tt.input)
			if h != tt.wantH || m != tt.wantMin || s != 0 {
				t.Errorf("parseTime(%q) = %d,%d,%d, want %d,%d,0", tt.input, h, m, s, tt.wantH, tt.wantMin)
			}
		})
	}
}

func TestDigestProductGroupAdapter(t *testing.T) {
	group := dbController.MailDigestProductGroup{
		Today:    []dbModel.Product{{ProductName: "Today"}},
		ThisWeek: []dbModel.Product{{ProductName: "Week"}, {ProductName: "Week2"}},
		NextWeek: []dbModel.Product{{ProductName: "Next"}},
	}
	adapter := &digestProductGroupAdapter{group: group}
	if len(adapter.GetToday()) != 1 || adapter.GetToday()[0].ProductName != "Today" {
		t.Errorf("GetToday() = %+v", adapter.GetToday())
	}
	if len(adapter.GetThisWeek()) != 2 {
		t.Errorf("GetThisWeek() = %d items, want 2", len(adapter.GetThisWeek()))
	}
	if len(adapter.GetNextWeek()) != 1 || adapter.GetNextWeek()[0].ProductName != "Next" {
		t.Errorf("GetNextWeek() = %+v", adapter.GetNextWeek())
	}
}

// newTelegramTestServer serves getMe, getUpdates and sendMessage for one bot token.
func newTelegramTestServer(t *testing.T, username string, updates string) (*httptest.Server, *int32, *int32) {
	t.Helper()
	var getMeHits, sendMessageHits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case len(r.URL.Path) > len("/getMe") && r.URL.Path[len(r.URL.Path)-len("/getMe"):] == "/getMe":
			atomic.AddInt32(&getMeHits, 1)
			fmt.Fprintf(w, `{"ok":true,"result":{"username":%q}}`, username)
		case len(r.URL.Path) > len("/getUpdates") && r.URL.Path[len(r.URL.Path)-len("/getUpdates"):] == "/getUpdates":
			w.Write([]byte(updates))
		case len(r.URL.Path) > len("/sendMessage") && r.URL.Path[len(r.URL.Path)-len("/sendMessage"):] == "/sendMessage":
			atomic.AddInt32(&sendMessageHits, 1)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server, &getMeHits, &sendMessageHits
}

func TestStartAndStopUserTelegramPoller(t *testing.T) {
	server, getMeHits, _ := newTelegramTestServer(t, "testbot", `{"ok":true,"result":[]}`)
	defer server.Close()

	mockRepos := repomocks.NewMockRepositoryContainer()
	mockRepos.Notifications.Users = []authentication.User{
		{Model: gorm.Model{ID: 1}, NotificationPreferences: authentication.NotificationPreferences{TelegramBotToken: "TOK"}},
	}
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{
		Telegram: configuration.TelegramConfiguration{PollerWorkers: 1},
	})
	nc.telegramClient = redirectClient(server)
	nc.telegramAPIBase = server.URL

	// Pool is nil: StartUserTelegramPoller bootstraps it from the repository.
	nc.StartUserTelegramPoller(9, "TOK")
	if nc.pollerPool == nil {
		t.Fatal("poller pool was not initialized")
	}
	nc.pollerPool.mu.RLock()
	_, registered := nc.pollerPool.users[9]
	poolSize := len(nc.pollerPool.users)
	nc.pollerPool.mu.RUnlock()
	if !registered {
		t.Error("user 9 was not registered in the pool")
	}
	if poolSize != 2 {
		t.Errorf("pool size = %d, want 2 (repository user + registered user)", poolSize)
	}
	if got := nc.GetUserTelegramBotUsername(9); got != "testbot" {
		t.Errorf("bot username = %q, want %q", got, "testbot")
	}
	if atomic.LoadInt32(getMeHits) == 0 {
		t.Error("getMe was never called to resolve the bot username")
	}

	nc.StopUserTelegramPoller(9)
	nc.pollerPool.mu.RLock()
	_, stillThere := nc.pollerPool.users[9]
	nc.pollerPool.mu.RUnlock()
	if stillThere {
		t.Error("user 9 was not removed from the pool")
	}
	if got := nc.GetUserTelegramBotUsername(9); got != "" {
		t.Errorf("bot username = %q, want empty after stop", got)
	}

	nc.StopTelegramPollerPool()
	if nc.pollerPool != nil {
		t.Error("poller pool was not cleaned up")
	}
	// Stopping with a nil pool is a no-op.
	nc.StopUserTelegramPoller(9)
	nc.StopTelegramPollerPool()
}

func TestProcessPoolUsersAndPollUser(t *testing.T) {
	updates := `{"ok":true,"result":[{"update_id":41,"message":{"chat":{"id":99},"text":"/start goodtoken"}}]}`
	server, _, sendMessageHits := newTelegramTestServer(t, "testbot", updates)
	defer server.Close()

	mockRepos := repomocks.NewMockRepositoryContainer()
	mockRepos.Notifications.User = authentication.User{Model: gorm.Model{ID: 7}}
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
	nc.telegramClient = redirectClient(server)
	nc.telegramAPIBase = server.URL
	nc.pollerPool = &telegramPollerPool{
		users:      map[uint]*pollerState{7: {botToken: "TOK"}},
		numWorkers: 1,
		nc:         nc,
	}

	nc.processPoolUsers(0)

	nc.pollerPool.mu.RLock()
	offset := nc.pollerPool.users[7].offset
	nc.pollerPool.mu.RUnlock()
	if offset != 42 {
		t.Errorf("offset = %d, want 42 (update_id 41 + 1)", offset)
	}
	if got := atomic.LoadInt32(sendMessageHits); got != 1 {
		t.Errorf("sendMessage hits = %d, want 1 (linked confirmation)", got)
	}
}

func TestPollUserErrorPaths(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
	nc.telegramAPIBase = "http://127.0.0.1:1" // unreachable
	nc.telegramClient = &http.Client{Timeout: time.Second}
	nc.pollerPool = &telegramPollerPool{users: map[uint]*pollerState{}, numWorkers: 1, nc: nc}
	state := &pollerState{botToken: "TOK"}

	t.Run("transport error", func(t *testing.T) {
		nc.pollUser(7, nc.telegramAPIBase+"/botTOK", state)
	})

	t.Run("unparseable response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))
		defer server.Close()
		nc.pollUser(7, server.URL+"/botTOK", state)
	})

	t.Run("update without message is skipped", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"ok":true,"result":[{"update_id":5}]}`))
		}))
		defer server.Close()
		nc.pollUser(7, server.URL+"/botTOK", state)
		if state.offset != 6 {
			t.Errorf("offset = %d, want 6", state.offset)
		}
	})
}

func TestHandleTelegramStartCommand(t *testing.T) {
	newNC := func(server *httptest.Server, mockRepos *repomocks.MockRepositoryContainer) *NotificationController {
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.telegramClient = server.Client()
		return nc
	}
	newServer := func(hits *int32) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(hits, 1)
			w.WriteHeader(http.StatusOK)
		}))
	}

	t.Run("non-start text ignored", func(t *testing.T) {
		var hits int32
		server := newServer(&hits)
		defer server.Close()
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newNC(server, mockRepos)
		if nc.handleTelegramStartCommand("hello", "99", server.URL+"/botTOK", 7) {
			t.Error("handleTelegramStartCommand(hello) = true, want false")
		}
		if atomic.LoadInt32(&hits) != 0 {
			t.Error("sendMessage called for a non-command message")
		}
	})

	t.Run("start without token sends instructions", func(t *testing.T) {
		var hits int32
		server := newServer(&hits)
		defer server.Close()
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newNC(server, mockRepos)
		if !nc.handleTelegramStartCommand("/start", "99", server.URL+"/botTOK", 7) {
			t.Error("handleTelegramStartCommand(/start) = false, want true")
		}
		if atomic.LoadInt32(&hits) != 1 {
			t.Errorf("sendMessage hits = %d, want 1", atomic.LoadInt32(&hits))
		}
	})

	t.Run("invalid token rejected", func(t *testing.T) {
		var hits int32
		server := newServer(&hits)
		defer server.Close()
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.User = authentication.User{Model: gorm.Model{ID: 999}} // token resolves to another user
		nc := newNC(server, mockRepos)
		if !nc.handleTelegramStartCommand("/start badtoken", "99", server.URL+"/botTOK", 7) {
			t.Error("handleTelegramStartCommand(/start badtoken) = false, want true")
		}
		if atomic.LoadInt32(&hits) != 1 {
			t.Errorf("sendMessage hits = %d, want 1 (rejection message)", atomic.LoadInt32(&hits))
		}
	})

	t.Run("unknown token rejected", func(t *testing.T) {
		var hits int32
		server := newServer(&hits)
		defer server.Close()
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.Err = gorm.ErrRecordNotFound
		nc := newNC(server, mockRepos)
		if !nc.handleTelegramStartCommand("/start badtoken", "99", server.URL+"/botTOK", 7) {
			t.Error("handleTelegramStartCommand = false, want true")
		}
	})

	t.Run("valid token links the chat", func(t *testing.T) {
		var hits int32
		server := newServer(&hits)
		defer server.Close()
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.User = authentication.User{Model: gorm.Model{ID: 7}}
		nc := newNC(server, mockRepos)
		if !nc.handleTelegramStartCommand("/start goodtoken", "99", server.URL+"/botTOK", 7) {
			t.Error("handleTelegramStartCommand = false, want true")
		}
		if atomic.LoadInt32(&hits) != 1 {
			t.Errorf("sendMessage hits = %d, want 1 (confirmation)", atomic.LoadInt32(&hits))
		}
	})

	t.Run("chat ID save failure reports error", func(t *testing.T) {
		var hits int32
		server := newServer(&hits)
		defer server.Close()
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.User = authentication.User{Model: gorm.Model{ID: 7}}
		nc := newNC(server, mockRepos)
		nc.NotificationRepo = &telegramLinkErrRepo{mockRepos.Notifications}
		if !nc.handleTelegramStartCommand("/start goodtoken", "99", server.URL+"/botTOK", 7) {
			t.Error("handleTelegramStartCommand = false, want true")
		}
		if atomic.LoadInt32(&hits) != 1 {
			t.Errorf("sendMessage hits = %d, want 1 (error message)", atomic.LoadInt32(&hits))
		}
	})
}

func TestResolveTelegramBotUsername(t *testing.T) {
	t.Run("success stores and persists the username", func(t *testing.T) {
		server, getMeHits, _ := newTelegramTestServer(t, "coolbot", `{"ok":true,"result":[]}`)
		defer server.Close()
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.telegramClient = redirectClient(server)

		nc.resolveTelegramBotUsername(server.URL+"/botTOK", 3)
		if atomic.LoadInt32(getMeHits) != 1 {
			t.Errorf("getMe hits = %d, want 1", atomic.LoadInt32(getMeHits))
		}
		if got := nc.GetUserTelegramBotUsername(3); got != "coolbot" {
			t.Errorf("bot username = %q, want %q", got, "coolbot")
		}
	})

	t.Run("unreachable API leaves the cache empty", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.telegramClient = &http.Client{Timeout: time.Second}
		nc.resolveTelegramBotUsername("http://127.0.0.1:1/botTOK", 3)
		if got := nc.GetUserTelegramBotUsername(3); got != "" {
			t.Errorf("bot username = %q, want empty", got)
		}
	})

	t.Run("unsuccessful getMe leaves the cache empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"ok":false}`))
		}))
		defer server.Close()
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.telegramClient = server.Client()
		nc.resolveTelegramBotUsername(server.URL+"/botTOK", 3)
		if got := nc.GetUserTelegramBotUsername(3); got != "" {
			t.Errorf("bot username = %q, want empty", got)
		}
	})
}

func TestProcessPendingInvitations(t *testing.T) {
	smtpConfig := &configuration.NotificationConfiguration{Interval: 24, SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}}
	invitation := dbModel.HouseholdInvitation{
		Model: gorm.Model{ID: 1}, Email: "invitee@example.com", HouseholdID: 2, InviterID: 8,
		Token: "tok", ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	t.Run("fetch error returns", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.Invitations = []dbModel.HouseholdInvitation{invitation}
		mockRepos.Notifications.User = authentication.User{Username: "inviter"}
		mockRepos.Notifications.Household = dbModel.Household{Name: "My Household"}
		mockRepos.Notifications.Err = gorm.ErrInvalidData
		nc := newExtraNotificationController(mockRepos, smtpConfig)

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processPendingInvitations(nc.newEmailProvider(), "http://example.com")
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (fetch error aborts the run)", emails)
		}
	})

	t.Run("no pending invitations", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, smtpConfig)

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processPendingInvitations(nc.newEmailProvider(), "http://example.com")
		if emails != 0 {
			t.Errorf("emails = %d, want 0 (no pending invitations, nothing to send)", emails)
		}
	})

	t.Run("pending invitation is dispatched", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.Invitations = []dbModel.HouseholdInvitation{invitation}
		mockRepos.Notifications.User = authentication.User{Username: "inviter"}
		mockRepos.Notifications.Household = dbModel.Household{Name: "My Household"}
		nc := newExtraNotificationController(mockRepos, smtpConfig)

		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.processPendingInvitations(nc.newEmailProvider(), "http://example.com")
		if emails != 1 {
			t.Errorf("emails = %d, want 1", emails)
		}
	})
}

func TestResolveInviterName(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})

	t.Run("lookup error falls back", func(t *testing.T) {
		mockRepos.Notifications.Err = gorm.ErrRecordNotFound
		if got := nc.resolveInviterName(8); got != "A household member" {
			t.Errorf("resolveInviterName() = %q, want fallback", got)
		}
		mockRepos.Notifications.Err = nil
	})

	t.Run("display name preferred", func(t *testing.T) {
		mockRepos.Notifications.User = authentication.User{Username: "user", DisplayName: "Friendly User"}
		if got := nc.resolveInviterName(8); got != "Friendly User" {
			t.Errorf("resolveInviterName() = %q, want %q", got, "Friendly User")
		}
	})

	t.Run("username fallback", func(t *testing.T) {
		mockRepos.Notifications.User = authentication.User{Username: "user"}
		if got := nc.resolveInviterName(8); got != "user" {
			t.Errorf("resolveInviterName() = %q, want %q", got, "user")
		}
	})
}

func TestResolveHouseholdName(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})

	t.Run("lookup error falls back", func(t *testing.T) {
		mockRepos.Notifications.Err = gorm.ErrRecordNotFound
		if got := nc.resolveHouseholdName(2); got != "Household #2" {
			t.Errorf("resolveHouseholdName() = %q, want %q", got, "Household #2")
		}
		mockRepos.Notifications.Err = nil
	})

	t.Run("household name returned", func(t *testing.T) {
		mockRepos.Notifications.Household = dbModel.Household{Name: "My Household"}
		if got := nc.resolveHouseholdName(2); got != "My Household" {
			t.Errorf("resolveHouseholdName() = %q, want %q", got, "My Household")
		}
	})
}

func TestDispatchPendingInvitation(t *testing.T) {
	invitation := &dbModel.HouseholdInvitation{
		Model: gorm.Model{ID: 1}, Email: "invitee@example.com", HouseholdID: 2, InviterID: 8,
		Token: "tok", ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	configured := &EmailNotificationProvider{Configuration: configuration.SMTPConfiguration{Host: "h", Port: 587}}

	t.Run("send failure marks invitation failed", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		recorder := &invitationMarkRecorder{MockNotificationRepository: mockRepos.Notifications}
		nc.NotificationRepo = recorder
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error { return errors.New("smtp down") }
		defer func() { emailSendFunc = origFunc }()

		nc.dispatchPendingInvitation(configured, invitation, "inviter", "Home", "http://example.com")
		if len(recorder.markFailed) != 1 || recorder.markFailed[0] != invitation.ID {
			t.Errorf("MarkInvitationSendFailed calls = %v, want [%d]", recorder.markFailed, invitation.ID)
		}
		if len(recorder.markSent) != 0 {
			t.Errorf("MarkInvitationSent calls = %v, want none on the failure path", recorder.markSent)
		}
	})

	t.Run("success marks invitation sent", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		recorder := &invitationMarkRecorder{MockNotificationRepository: mockRepos.Notifications}
		nc.NotificationRepo = recorder
		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		nc.dispatchPendingInvitation(configured, invitation, "inviter", "Home", "http://example.com")
		if emails != 1 {
			t.Errorf("emails = %d, want 1", emails)
		}
		if len(recorder.markSent) != 1 || recorder.markSent[0] != invitation.ID {
			t.Errorf("MarkInvitationSent calls = %v, want [%d]", recorder.markSent, invitation.ID)
		}
		if len(recorder.markFailed) != 0 {
			t.Errorf("MarkInvitationSendFailed calls = %v, want none on the success path", recorder.markFailed)
		}
	})
}

func TestSendTelegram(t *testing.T) {
	product := &dbModel.Product{
		Model:       gorm.Model{ID: 1},
		ProductName: "Milk",
		ExpireAt:    time.Now().Add(48 * time.Hour),
	}

	t.Run("recipient disabled returns false", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		if nc.sendTelegram(product, &models.NotificationRecipientInfo{}) {
			t.Error("sendTelegram() = true, want false for a disabled recipient")
		}
	})

	t.Run("send success returns true", func(t *testing.T) {
		var hits int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.telegramClient = redirectClient(server)
		pref := &models.NotificationRecipientInfo{TelegramEnabled: true, TelegramChatID: "77", TelegramBotToken: "BT"}
		if !nc.sendTelegram(product, pref) {
			t.Error("sendTelegram() = false, want true")
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Errorf("telegram hits = %d, want 1", got)
		}
	})

	t.Run("send failure returns false", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.telegramClient = redirectClient(server)
		pref := &models.NotificationRecipientInfo{TelegramEnabled: true, TelegramChatID: "77", TelegramBotToken: "BT"}
		if nc.sendTelegram(product, pref) {
			t.Error("sendTelegram() = true, want false on API error")
		}
	})
}

func TestSendWebPush(t *testing.T) {
	product := &dbModel.Product{
		Model:       gorm.Model{ID: 1},
		ProductName: "Milk",
		ExpireAt:    time.Now().Add(48 * time.Hour),
	}

	t.Run("recipient disabled returns false", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		if nc.sendWebPush(product, &models.NotificationRecipientInfo{}) {
			t.Error("sendWebPush() = true, want false for a disabled recipient")
		}
	})

	t.Run("invalid subscription returns false", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		pref := &models.NotificationRecipientInfo{WebPushEnabled: true, WebPushSubscriptionJSON: "{not json"}
		if nc.sendWebPush(product, pref) {
			t.Error("sendWebPush() = true, want false for an invalid subscription")
		}
	})

	t.Run("send success returns true", func(t *testing.T) {
		var hits int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		subscriptionJSON, publicKey, privateKey := newTestWebPushSubscription(t, server.URL)
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		nc.NotificationRepo = &vapidKeyRepo{MockNotificationRepository: mockRepos.Notifications, publicKey: publicKey, privateKey: privateKey}
		pref := &models.NotificationRecipientInfo{WebPushEnabled: true, WebPushSubscriptionJSON: subscriptionJSON}
		if !nc.sendWebPush(product, pref) {
			t.Error("sendWebPush() = false, want true")
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Errorf("push endpoint hits = %d, want 1", got)
		}
	})
}

func TestMarkNotified(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		if !nc.markNotified(1) {
			t.Error("markNotified() = false, want true")
		}
	})

	t.Run("repository error returns false", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		mockRepos.Notifications.Err = gorm.ErrInvalidData
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		if nc.markNotified(1) {
			t.Error("markNotified() = true, want false on repository error")
		}
	})
}

func TestSendViaProvider(t *testing.T) {
	product := &dbModel.Product{Model: gorm.Model{ID: 1}, ProductName: "Milk", ExpireAt: time.Now().Add(-time.Hour)}

	t.Run("email disabled returns nil", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		provider := &EmailNotificationProvider{Configuration: configuration.SMTPConfiguration{Host: "h", Port: 587}}
		pref := &models.NotificationRecipientInfo{EmailAddress: "a@example.com"}
		if err := nc.sendViaProvider(provider, product, pref); err != nil {
			t.Errorf("sendViaProvider() = %v, want nil", err)
		}
	})

	t.Run("email enabled sends", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		emails := 0
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			emails++
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		provider := &EmailNotificationProvider{Configuration: configuration.SMTPConfiguration{Host: "h", Port: 587}}
		pref := &models.NotificationRecipientInfo{EmailEnabled: true, EmailAddress: "a@example.com"}
		if err := nc.sendViaProvider(provider, product, pref); err != nil {
			t.Errorf("sendViaProvider() = %v, want nil", err)
		}
		if emails != 1 {
			t.Errorf("emails = %d, want 1", emails)
		}
	})

	t.Run("ntfy enabled reaches the provider", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		logger := zerolog.Nop()
		provider := &NtfyNotificationProvider{
			Configuration: configuration.NtfyConfiguration{URL: server.URL, Topic: "t"},
			Logger:        &logger,
			HTTPClient:    &http.Client{Timeout: time.Second},
		}
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		pref := &models.NotificationRecipientInfo{NtfyEnabled: true, NtfyURL: server.URL, NtfyTopic: "t"}
		if err := nc.sendViaProvider(provider, product, pref); err != nil {
			t.Errorf("sendViaProvider() = %v, want nil", err)
		}
	})

	t.Run("unknown provider type returns nil", func(t *testing.T) {
		mockRepos := repomocks.NewMockRepositoryContainer()
		nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
		provider := &unknownProvider{}
		pref := &models.NotificationRecipientInfo{}
		if err := nc.sendViaProvider(provider, product, pref); err != nil {
			t.Errorf("sendViaProvider() = %v, want nil", err)
		}
		if provider.sent {
			t.Error("unknown provider's SendNotification was called")
		}
	})
}

func TestIsWithinNotificationThresholdZeroExpiry(t *testing.T) {
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := newExtraNotificationController(mockRepos, &configuration.NotificationConfiguration{})
	product := &dbModel.Product{ProductName: "NoDate"}
	pref := &models.NotificationRecipientInfo{NotificationThresholdDays: 3}
	if nc.isWithinNotificationThreshold(product, pref) {
		t.Error("isWithinNotificationThreshold() = true, want false for a product without expiry")
	}
}
