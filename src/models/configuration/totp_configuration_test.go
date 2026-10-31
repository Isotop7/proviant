package configuration

import (
	"encoding/base64"
	"testing"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/util"
)

func twoFactorConfig(encryptionKey string) *ProviantConfiguration {
	return &ProviantConfiguration{
		Server: ServerConfiguration{
			Authentication: AuthenticationConfiguration{
				TokenPassword: "secret-key-12345",
			},
			TwoFactor: TwoFactorConfiguration{
				Enabled:           true,
				EncryptionKey:     encryptionKey,
				Issuer:            "Proviant",
				RecoveryCodeCount: 10,
			},
		},
	}
}

func TestValidateTwoFactorConfiguration(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString(make([]byte, util.TwoFactorEncryptionKeyBytes))

	tests := []struct {
		name    string
		config  *ProviantConfiguration
		wantErr error
	}{
		{
			name: "disabled feature needs no key",
			config: func() *ProviantConfiguration {
				config := twoFactorConfig("")
				config.Server.TwoFactor.Enabled = false
				return config
			}(),
			wantErr: nil,
		},
		{
			name:    "valid configuration",
			config:  twoFactorConfig(validKey),
			wantErr: nil,
		},
		{
			name:    "empty key",
			config:  twoFactorConfig(""),
			wantErr: errors.ErrServerEmptyTwoFactorKey,
		},
		{
			name:    "shipped placeholder key",
			config:  twoFactorConfig(util.RetiredTwoFactorEncryptionKey),
			wantErr: errors.ErrServerDefaultTwoFactorKey,
		},
		{
			name:    "key that is not base64",
			config:  twoFactorConfig("not base64!!"),
			wantErr: errors.ErrServerInvalidTwoFactorKey,
		},
		{
			name:    "key of the wrong length",
			config:  twoFactorConfig(base64.StdEncoding.EncodeToString(make([]byte, 16))),
			wantErr: errors.ErrServerInvalidTwoFactorKey,
		},
		{
			name: "key equal to the token password",
			config: func() *ProviantConfiguration {
				config := twoFactorConfig(validKey)
				config.Server.Authentication.TokenPassword = validKey
				return config
			}(),
			wantErr: errors.ErrServerTwoFactorKeyEqualsTokenPassword,
		},
		{
			name: "recovery code count out of range",
			config: func() *ProviantConfiguration {
				config := twoFactorConfig(validKey)
				config.Server.TwoFactor.RecoveryCodeCount = 0
				return config
			}(),
			wantErr: errors.ErrServerInvalidTwoFactorRecoveryCodeCount,
		},
		{
			name: "recovery code count too high",
			config: func() *ProviantConfiguration {
				config := twoFactorConfig(validKey)
				config.Server.TwoFactor.RecoveryCodeCount = 51
				return config
			}(),
			wantErr: errors.ErrServerInvalidTwoFactorRecoveryCodeCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateTwoFactorConfiguration()
			if err != tt.wantErr {
				t.Errorf("ValidateTwoFactorConfiguration() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTwoFactorConfigurationAcceptsDisabledFeatureWithEmptyKey(t *testing.T) {
	config := twoFactorConfig("")
	config.Server.TwoFactor.Enabled = false

	if err := config.ValidateTwoFactorConfiguration(); err != nil {
		t.Errorf("ValidateTwoFactorConfiguration() error = %v, want nil", err)
	}
}
