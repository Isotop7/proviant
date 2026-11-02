package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Isotop7/proviant/metrics"
	"github.com/Isotop7/proviant/models"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	repomocks "github.com/Isotop7/proviant/testutil/mocks"
	"github.com/Isotop7/proviant/util"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
	gomail "gopkg.in/mail.v2"
)

// familyValue returns the summed value of every series of a metric family, or
// zero when the family is not exposed.
func familyValue(t *testing.T, registry *prometheus.Registry, name string) float64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		total := 0.0
		for _, series := range family.GetMetric() {
			if counter := series.GetCounter(); counter != nil {
				total += counter.GetValue()
				continue
			}
			total += series.GetGauge().GetValue()
		}
		return total
	}
	return 0
}

// labeledValue returns the value of the series of a metric family that carries
// the given label value, or zero when no such series exists. The collectors
// are shared by every test in the process, so the tests assert on the
// difference between two readings instead of on absolute values.
func labeledValue(t *testing.T, registry *prometheus.Registry, name, labelValue string) float64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, series := range family.GetMetric() {
			for _, pair := range series.GetLabel() {
				if pair.GetValue() != labelValue {
					continue
				}
				if counter := series.GetCounter(); counter != nil {
					return counter.GetValue()
				}
				return series.GetGauge().GetValue()
			}
		}
	}
	return 0
}

// exposedNames returns the metric names the registry currently exposes.
func exposedNames(t *testing.T, registry *prometheus.Registry) map[string]bool {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	names := make(map[string]bool, len(families))
	for _, family := range families {
		names[family.GetName()] = true
	}
	return names
}

// failingEmailProvider stands in for an SMTP server that refuses the message.
type failingEmailProvider struct{}

func (p *failingEmailProvider) GetProviderType() string { return util.ProviderEmail }
func (p *failingEmailProvider) IsConfigured() bool      { return true }
func (p *failingEmailProvider) SendNotification(*dbModel.Product, any, string) error {
	return errors.New("smtp refused the message")
}

// failingNtfyProvider stands in for an ntfy server that rejects the message.
type failingNtfyProvider struct{}

func (p *failingNtfyProvider) GetProviderType() string { return util.ProviderNtfy }
func (p *failingNtfyProvider) IsConfigured() bool      { return true }
func (p *failingNtfyProvider) SendNotification(*dbModel.Product, any, string) error {
	return errors.New("ntfy rejected the message")
}

// notifiedRecorder records which products the controller marked as notified.
type notifiedRecorder struct {
	*repomocks.MockNotificationRepository
	notified []uint
}

func (r *notifiedRecorder) SetProductNotifiedAt(productID uint) error {
	r.notified = append(r.notified, productID)
	return r.MockNotificationRepository.SetProductNotifiedAt(productID)
}

// preferencesErrRepo fails only the preferences lookup.
type preferencesErrRepo struct {
	*repomocks.MockNotificationRepository
}

func (r *preferencesErrRepo) GetHouseholdMembersNotificationPreferences(uint) ([]models.NotificationRecipientInfo, error) {
	return nil, errors.New("preferences unavailable")
}

// TestBackupWorkerExportsMetricsAfterRun proves the household backup job
// shows up in the registry: the operator has to be able to see that the last
// backup is missing or stale.
func TestBackupWorkerExportsMetricsAfterRun(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics.RegisterCollectors(registry)

	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	config := &configuration.BackupConfiguration{
		Enabled:       true,
		IntervalHours: 24,
		Directory:     t.TempDir(),
		Retention:     7,
	}
	logger := zerolog.Nop()
	controller := NewBackupController(&logger, config, db)

	attemptsBefore := labeledValue(t, registry, "proviant_worker_attempts_total", util.WorkerBackup)
	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	attemptsAfter := labeledValue(t, registry, "proviant_worker_attempts_total", util.WorkerBackup)

	if attemptsAfter != attemptsBefore+1 {
		t.Errorf("backup attempts = %v, want %v", attemptsAfter, attemptsBefore+1)
	}

	names := exposedNames(t, registry)
	for _, name := range []string{
		"proviant_worker_attempts_total",
		"proviant_worker_last_success_timestamp_seconds",
		"proviant_backup_age_seconds",
	} {
		if !names[name] {
			t.Errorf("metric %q is missing after the backup worker ran", name)
		}
	}
	if got := labeledValue(t, registry, "proviant_worker_last_success_timestamp_seconds", util.WorkerBackup); got <= 0 {
		t.Errorf("backup last success = %v, want a unix timestamp", got)
	}
	if got := familyValue(t, registry, "proviant_backup_age_seconds"); got < 0 {
		t.Errorf("backup age = %v, want the age of the written backup", got)
	}
}

// TestMailDigestWorkerExportsMetricsAfterRun proves a notification worker run
// lands in the worker counters, so a scheduler that stops running is visible.
func TestMailDigestWorkerExportsMetricsAfterRun(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics.RegisterCollectors(registry)

	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{Interval: 24},
		NotificationRepo: mockRepos.Notifications,
		ProductRepo:      mockRepos.Products,
	}

	attemptsBefore := labeledValue(t, registry, "proviant_worker_attempts_total", util.WorkerMailDigest)
	nc.processMailDigestEmails("http://localhost:5114", "08:00")
	attemptsAfter := labeledValue(t, registry, "proviant_worker_attempts_total", util.WorkerMailDigest)

	if attemptsAfter != attemptsBefore+1 {
		t.Errorf("digest attempts = %v, want %v", attemptsAfter, attemptsBefore+1)
	}
	if got := labeledValue(t, registry, "proviant_worker_last_success_timestamp_seconds", util.WorkerMailDigest); got <= 0 {
		t.Errorf("digest last success = %v, want a unix timestamp", got)
	}
}

// TestGenerateNotificationsRetriesAfterAFailedDelivery proves a product whose
// only channel failed stays pending. Marking it notified on a skipped channel
// would drop the delivery for good.
func TestGenerateNotificationsRetriesAfterAFailedDelivery(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{{NtfyEnabled: true}}
	repo := &notifiedRecorder{MockNotificationRepository: mockRepos.Notifications}
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{Interval: 24},
		NotificationRepo: repo,
		ProductRepo:      mockRepos.Products,
	}
	nc.Providers = []NotificationProvider{&failingNtfyProvider{}}

	products := []dbModel.Product{{Model: gorm.Model{ID: 7}, ProductName: "Milk", ExpireAt: time.Now().Add(-time.Hour)}}
	nc.generateNotifications(&products)

	if len(repo.notified) != 0 {
		t.Errorf("notified products = %v, want none: the only channel failed", repo.notified)
	}
}

// TestGenerateNotificationsMarksAfterADelivery proves a delivered notification
// marks the product, so the sweep does not notify the household again.
func TestGenerateNotificationsMarksAfterADelivery(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{{
		EmailEnabled: true,
		EmailAddress: "member@example.com",
	}}
	repo := &notifiedRecorder{MockNotificationRepository: mockRepos.Notifications}
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{Interval: 24},
		NotificationRepo: repo,
		ProductRepo:      mockRepos.Products,
	}
	nc.Providers = []NotificationProvider{
		&EmailNotificationProvider{Configuration: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}},
	}

	originalSend := emailSendFunc
	emailSendFunc = func(*gomail.Dialer, *gomail.Message) error { return nil }
	defer func() { emailSendFunc = originalSend }()

	products := []dbModel.Product{{Model: gorm.Model{ID: 8}, ProductName: "Milk", ExpireAt: time.Now().Add(-time.Hour)}}
	nc.generateNotifications(&products)

	if len(repo.notified) != 1 || repo.notified[0] != products[0].ID {
		t.Errorf("notified products = %v, want [%d]", repo.notified, products[0].ID)
	}
}

// TestGenerateNotificationsClearsProductsNobodyWants proves a household where
// no member wants a notification leaves the product handled, so it is not
// re-queued on every sweep.
func TestGenerateNotificationsClearsProductsNobodyWants(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{{EmailAddress: "member@example.com"}}
	repo := &notifiedRecorder{MockNotificationRepository: mockRepos.Notifications}
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{Interval: 24},
		NotificationRepo: repo,
		ProductRepo:      mockRepos.Products,
	}
	nc.Providers = []NotificationProvider{
		&EmailNotificationProvider{Configuration: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}},
	}

	products := []dbModel.Product{{Model: gorm.Model{ID: 9}, ProductName: "Milk", ExpireAt: time.Now().Add(-time.Hour)}}
	nc.generateNotifications(&products)

	if len(repo.notified) != 1 || repo.notified[0] != products[0].ID {
		t.Errorf("notified products = %v, want [%d]", repo.notified, products[0].ID)
	}
}

// TestGenerateNotificationsKeepsProductPendingForOneMember proves one member
// opting out does not mark the product: another member's failed delivery still
// has to be retried.
func TestGenerateNotificationsKeepsProductPendingForOneMember(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{
		{EmailAddress: "opted-out@example.com"},
		{NtfyEnabled: true},
	}
	repo := &notifiedRecorder{MockNotificationRepository: mockRepos.Notifications}
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{Interval: 24},
		NotificationRepo: repo,
		ProductRepo:      mockRepos.Products,
	}
	nc.Providers = []NotificationProvider{&failingNtfyProvider{}}

	products := []dbModel.Product{{Model: gorm.Model{ID: 10}, ProductName: "Milk", ExpireAt: time.Now().Add(-time.Hour)}}
	nc.generateNotifications(&products)

	if len(repo.notified) != 0 {
		t.Errorf("notified products = %v, want none: one member's delivery failed", repo.notified)
	}
}

// TestGenerateNotificationsReportsAPartialFailure proves a sweep that could
// not read a household's preferences reports a failure: the household got no
// notification, and counting the sweep as a success would hide it.
func TestGenerateNotificationsReportsAPartialFailure(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{Interval: 24},
		NotificationRepo: &preferencesErrRepo{mockRepos.Notifications},
		ProductRepo:      mockRepos.Products,
	}

	products := []dbModel.Product{{Model: gorm.Model{ID: 11}, ProductName: "Milk", ExpireAt: time.Now().Add(-time.Hour)}}
	if nc.generateNotifications(&products) {
		t.Error("generateNotifications() = true, want false: the preferences lookup failed")
	}
}

// TestGenerateNotificationsReportsSuccess proves a sweep that handled every
// product reports a success.
func TestGenerateNotificationsReportsSuccess(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{{NtfyEnabled: true}}
	repo := &notifiedRecorder{MockNotificationRepository: mockRepos.Notifications}
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{Interval: 24},
		NotificationRepo: repo,
		ProductRepo:      mockRepos.Products,
	}
	nc.Providers = []NotificationProvider{&failingNtfyProvider{}}

	products := []dbModel.Product{{Model: gorm.Model{ID: 12}, ProductName: "Milk", ExpireAt: time.Now().Add(-time.Hour)}}
	if !nc.generateNotifications(&products) {
		t.Error("generateNotifications() = false, want true: the sweep completed")
	}
}

// TestNotificationDeliveryExportsProviderOutcomes proves a failed send is
// counted per provider: a broken SMTP configuration has to show up somewhere
// the operator looks.
func TestNotificationDeliveryExportsProviderOutcomes(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics.RegisterCollectors(registry)

	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{Interval: 24},
		NotificationRepo: mockRepos.Notifications,
		ProductRepo:      mockRepos.Products,
	}

	recipient := &models.NotificationRecipientInfo{EmailEnabled: true, EmailAddress: "member@example.com"}
	attemptsBefore := labeledValue(t, registry, "proviant_notification_attempts_total", util.ProviderEmail)
	failuresBefore := labeledValue(t, registry, "proviant_notification_failures_total", util.ProviderEmail)

	attempted, err := nc.sendViaProvider(&failingEmailProvider{}, &dbModel.Product{}, recipient, "en")
	if err == nil {
		t.Fatal("sendViaProvider() = nil, want the provider error")
	}
	if !attempted {
		t.Error("sendViaProvider() did not attempt a send")
	}

	attemptsAfter := labeledValue(t, registry, "proviant_notification_attempts_total", util.ProviderEmail)
	failuresAfter := labeledValue(t, registry, "proviant_notification_failures_total", util.ProviderEmail)

	if attemptsAfter != attemptsBefore+1 {
		t.Errorf("email attempts = %v, want %v", attemptsAfter, attemptsBefore+1)
	}
	if failuresAfter != failuresBefore+1 {
		t.Errorf("email failures = %v, want %v", failuresAfter, failuresBefore+1)
	}
}
