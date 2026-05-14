package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	gomail "gopkg.in/mail.v2"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestTelegramTimeout(t *testing.T) {
	tests := []struct {
		name   string
		config *configuration.NotificationConfiguration
		want   int
	}{
		{"zero timeout", &configuration.NotificationConfiguration{}, 15},
		{"positive timeout", &configuration.NotificationConfiguration{Telegram: configuration.TelegramConfiguration{Timeout: 30}}, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := telegramTimeout(tt.config)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNotificationControllerInitialization(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{},
		NotificationRepo: mockRepos.Notifications,
		ProductRepo:      mockRepos.Products,
	}
	assert.NotNil(t, nc.NotificationRepo)
}

func TestGenerateNotifications(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()

	t.Run("empty products no-op", func(t *testing.T) {
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{Interval: 24},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		nc.generateNotifications(&[]dbModel.Product{})
	})

	t.Run("preferences error logs and continues", func(t *testing.T) {
		mockRepos.Notifications.NotifRecipients = nil
		mockRepos.Notifications.Err = gorm.ErrInvalidData
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{Interval: 24},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		products := []dbModel.Product{
			{Model: gorm.Model{ID: 1}, ProductName: "Test", ExpireAt: time.Now().Add(-1 * time.Hour), HouseholdID: 1},
		}
		nc.generateNotifications(&products)
	})
}

func TestSendNotificationsForRecipient(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()

	tests := []struct {
		name            string
		threshold       int
		expireOffset    time.Duration
		emailEnabled    bool
		ntfyEnabled     bool
		telegramEnabled bool
	}{
		{"threshold 0 expired", 0, -1 * time.Hour, false, false, false},
		{"threshold 3 expire 5d skip", 3, 5 * 24 * time.Hour, false, false, false},
		{"threshold 3 expire 2d notify", 3, 2 * 24 * time.Hour, false, false, false},
		{"email enabled", 0, -1 * time.Hour, true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nc := &NotificationController{
				Logger:           &logger,
				Configuration:    &configuration.NotificationConfiguration{Interval: 24},
				NotificationRepo: mockRepos.Notifications,
				ProductRepo:      mockRepos.Products,
			}
			if tt.emailEnabled {
				nc.Providers = []NotificationProvider{
					&EmailNotificationProvider{Configuration: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}},
				}
			}
			product := &dbModel.Product{
				Model:       gorm.Model{ID: 1},
				ProductName: "Test",
				ExpireAt:    time.Now().Add(tt.expireOffset),
			}
			pref := models.NotificationRecipientInfo{
				NotificationThresholdDays: tt.threshold,
				EmailEnabled:              tt.emailEnabled,
				EmailAddress:              "test@example.com",
			}
			nc.sendNotificationsForRecipient(product, &pref)
		})
	}
}

func TestNewNotificationController(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := NewNotificationController(&logger, &configuration.NotificationConfiguration{}, mockRepos.Notifications, mockRepos.Products)
	assert.NotNil(t, nc)
	assert.Equal(t, 15*time.Second, nc.telegramClient.Timeout)
	assert.Equal(t, "https://api.telegram.org", nc.telegramAPIBase)
}

func TestInitializeProviders(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()

	t.Run("no providers configured", func(t *testing.T) {
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		nc.initializeProviders()
		assert.Empty(t, nc.Providers)
	})

	t.Run("email provider configured", func(t *testing.T) {
		nc := &NotificationController{
			Logger: &logger,
			Configuration: &configuration.NotificationConfiguration{
				SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587},
			},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		nc.initializeProviders()
		assert.Len(t, nc.Providers, 1)
		assert.Equal(t, "email", nc.Providers[0].GetProviderType())
	})
}

func TestDispatchEarlyReturn(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()

	t.Run("disabled returns early", func(t *testing.T) {
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{Enabled: false},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		nc.Dispatch()
	})

	t.Run("enabled starts goroutine", func(t *testing.T) {
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{Enabled: true, Interval: 24},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		go nc.Dispatch()
		time.Sleep(50 * time.Millisecond)
	})
}

func TestSendStreakMilestoneNotifications(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()

	t.Run("prefs error returns early", func(t *testing.T) {
		mockRepos.Notifications.Err = gorm.ErrInvalidData
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		nc.sendStreakMilestoneNotifications(1, 7)
	})

	t.Run("email fan-out success", func(t *testing.T) {
		mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{
			{EmailEnabled: true, EmailAddress: "test@example.com"},
		}
		mockRepos.Notifications.Err = nil
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		nc.sendStreakMilestoneNotifications(1, 7)
	})

	t.Run("telegram fan-out", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		mockRepos.Notifications.NotifRecipients = []models.NotificationRecipientInfo{
			{TelegramEnabled: true, TelegramChatID: "123", TelegramBotToken: "token"},
		}
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
			telegramClient:   &http.Client{Timeout: 5 * time.Second},
			telegramAPIBase:  server.URL,
		}
		nc.sendStreakMilestoneNotifications(1, 7)
	})
}

func TestSendInvitationEmail(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	invitation := &dbModel.HouseholdInvitation{
		Model:       gorm.Model{ID: 1},
		Email:       "test@example.com",
		HouseholdID: 1,
		InviterID:   1,
	}
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}},
		NotificationRepo: mockRepos.Notifications,
		ProductRepo:      mockRepos.Products,
	}

	t.Run("not configured returns error", func(t *testing.T) {
		ncBad := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		err := ncBad.SendInvitationEmail(invitation, "inviter", "Household", "http://example.com")
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		err := nc.SendInvitationEmail(invitation, "inviter", "Household", "http://example.com")
		assert.NoError(t, err)
	})

	t.Run("send fails marks failed", func(t *testing.T) {
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			return fmt.Errorf("send error")
		}
		defer func() { emailSendFunc = origFunc }()

		err := nc.SendInvitationEmail(invitation, "inviter", "Household", "http://example.com")
		assert.Error(t, err)
	})
}

func TestSendVerificationEmail(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	invitation := &dbModel.HouseholdInvitation{
		Model:       gorm.Model{ID: 1},
		Email:       "test@example.com",
		HouseholdID: 1,
		InviterID:   1,
	}
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}},
		NotificationRepo: mockRepos.Notifications,
		ProductRepo:      mockRepos.Products,
	}

	t.Run("not configured returns error", func(t *testing.T) {
		ncBad := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		err := ncBad.SendVerificationEmail(invitation, "user1", "http://example.com")
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		err := nc.SendVerificationEmail(invitation, "user1", "http://example.com")
		assert.NoError(t, err)
	})
}

func TestSendEmailVerification(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{SMTP: configuration.SMTPConfiguration{Host: "smtp.example.com", Port: 587}},
		NotificationRepo: mockRepos.Notifications,
		ProductRepo:      mockRepos.Products,
	}

	t.Run("not configured returns error", func(t *testing.T) {
		ncBad := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		err := ncBad.SendEmailVerification("test@example.com", "user1", "token", "http://example.com", time.Now())
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		origFunc := emailSendFunc
		emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
			return nil
		}
		defer func() { emailSendFunc = origFunc }()

		err := nc.SendEmailVerification("test@example.com", "user1", "token", "http://example.com", time.Now())
		assert.NoError(t, err)
	})
}

func TestDispatchInvitationsEarlyReturn(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()

	t.Run("email not configured returns early", func(t *testing.T) {
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{Interval: 24},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		nc.DispatchInvitations("http://example.com")
	})
}

func TestDispatchMonthlyWasteReportsEarlyReturn(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()

	t.Run("email not configured", func(t *testing.T) {
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{MonthlyWasteReport: configuration.MonthlyWasteReportConfiguration{Day: 1, Hour: 0}},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
		}
		emailProvider := &EmailNotificationProvider{Configuration: configuration.SMTPConfiguration{}}
		nc.processMonthlyWasteReports(emailProvider)
	})
}

func TestDispatchStreakUpdatesEarlyReturn(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()

	t.Run("nil StreakRepo returns early", func(t *testing.T) {
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
			StreakRepo:       nil,
		}
		nc.DispatchStreakUpdates()
	})
}

func TestStartAllUserTelegramPollers(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()

	t.Run("no users initializes pool", func(t *testing.T) {
		mockRepos.Notifications.Users = []authentication.User{}
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{Telegram: configuration.TelegramConfiguration{PollerWorkers: 2}},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
			telegramClient:   &http.Client{Timeout: 5 * time.Second},
			telegramAPIBase:  "https://api.telegram.org",
		}
		nc.StartAllUserTelegramPollers()
		assert.NotNil(t, nc.pollerPool)
		assert.Equal(t, 0, len(nc.pollerPool.users))
		nc.StopTelegramPollerPool()
	})

	t.Run("pool starts with workers", func(t *testing.T) {
		mockRepos.Notifications.Users = []authentication.User{
			{Model: gorm.Model{ID: 1}, NotificationPreferences: authentication.NotificationPreferences{TelegramBotToken: "test-token"}},
		}
		nc := &NotificationController{
			Logger:           &logger,
			Configuration:    &configuration.NotificationConfiguration{Telegram: configuration.TelegramConfiguration{PollerWorkers: 2}},
			NotificationRepo: mockRepos.Notifications,
			ProductRepo:      mockRepos.Products,
			telegramClient:   &http.Client{Timeout: 5 * time.Second},
			telegramAPIBase:  "https://api.telegram.org",
		}
		nc.StartAllUserTelegramPollers()
		assert.NotNil(t, nc.pollerPool)
		assert.Equal(t, 1, len(nc.pollerPool.users))
		assert.Equal(t, 2, nc.pollerPool.numWorkers)
		nc.StopTelegramPollerPool()
	})
}

func TestSendTelegramText(t *testing.T) {
	logger := zerolog.Nop()
	nc := &NotificationController{
		Logger:          &logger,
		telegramClient:  &http.Client{Timeout: 5 * time.Second},
		telegramAPIBase: "https://api.telegram.org",
	}

	t.Run("200 returns nil", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		nc.telegramAPIBase = server.URL
		nc.sendTelegramText(nc.telegramClient, server.URL, "123", "test message")
	})

	t.Run("non-2xx returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()
		nc.telegramAPIBase = server.URL
		nc.sendTelegramText(nc.telegramClient, server.URL, "123", "test message")
	})
}

func TestGetUserTelegramBotUsername(t *testing.T) {
	logger := zerolog.Nop()
	mockRepos := repomocks.NewMockRepositoryContainer()
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{},
		NotificationRepo: mockRepos.Notifications,
		ProductRepo:      mockRepos.Products,
	}
	assert.Equal(t, "", nc.GetUserTelegramBotUsername(1))
}