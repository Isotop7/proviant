package controllers

import (
	"net/http"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/rs/zerolog"
)

func TestTelegramTimeout(t *testing.T) {
	tests := []struct {
		name string
		config *configuration.NotificationConfiguration
		want  int
	}{
		{"zero timeout", &configuration.NotificationConfiguration{}, 15},
		{"positive timeout", &configuration.NotificationConfiguration{Telegram: configuration.TelegramConfiguration{Timeout: 30}}, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := telegramTimeout(tt.config)
			if got != tt.want {
				t.Errorf("telegramTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotificationControllerInitialization(t *testing.T) {
	logger := zerolog.Nop()
	mockRepo := repomocks.NewMockRepositoryContainer().Notifications
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{},
		NotificationRepo: mockRepo,
	}
	if nc.NotificationRepo == nil {
		t.Error("Expected notification repository to be set, got nil")
	}
}

func TestGenerateNotifications_EmptyProducts(t *testing.T) {
	logger := zerolog.Nop()
	mockRepo := repomocks.NewMockRepositoryContainer().Notifications
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{Interval: 24},
		NotificationRepo: mockRepo,
	}
	nc.generateNotifications(&[]dbModel.Product{})
}

func TestStartStopTelegramPoller(t *testing.T) {
	logger := zerolog.Nop()
	mockRepo := repomocks.NewMockRepositoryContainer().Notifications
	nc := &NotificationController{
		Logger:           &logger,
		Configuration:    &configuration.NotificationConfiguration{},
		NotificationRepo: mockRepo,
		telegramClient:   &http.Client{Timeout: 5 * time.Second},
		telegramAPIBase:  "https://api.telegram.org",
	}

	nc.StartUserTelegramPoller(1, "invalidtoken")
	time.Sleep(50 * time.Millisecond)
	nc.StopUserTelegramPoller(1)
	_, ok := nc.pollerCancels.Load(uint(1))
	if ok {
		t.Error("expected poller cancel to be cleared after stop")
	}
}
