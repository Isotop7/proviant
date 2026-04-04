package authentication

import "testing"

func TestUserIsValid(t *testing.T) {
	tests := []struct {
		name         string
		user         User
		skipPassword bool
		wantErr      bool
	}{
		{
			name: "valid user with password",
			user: User{
				ID:          1,
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "test@example.com",
			},
			skipPassword: false,
			wantErr:      false,
		},
		{
			name: "valid user skipping password check",
			user: User{
				ID:          1,
				Username:    "testuser",
				MailAddress: "test@example.com",
			},
			skipPassword: true,
			wantErr:      false,
		},
		{
			name: "invalid user ID - zero",
			user: User{
				ID:          0,
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "test@example.com",
			},
			skipPassword: false,
			wantErr:      true,
		},
		{
			name: "invalid user ID - negative equivalent",
			user: User{
				ID:          0,
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "test@example.com",
			},
			skipPassword: false,
			wantErr:      true,
		},
		{
			name: "empty username",
			user: User{
				ID:          1,
				Username:    "",
				Password:    "password123",
				MailAddress: "test@example.com",
			},
			skipPassword: false,
			wantErr:      true,
		},
		{
			name: "password too short when not skipping",
			user: User{
				ID:          1,
				Username:    "testuser",
				Password:    "short",
				MailAddress: "test@example.com",
			},
			skipPassword: false,
			wantErr:      true,
		},
		{
			name: "password too short but skipped",
			user: User{
				ID:          1,
				Username:    "testuser",
				Password:    "short",
				MailAddress: "test@example.com",
			},
			skipPassword: true,
			wantErr:      false,
		},
		{
			name: "empty password when not skipping",
			user: User{
				ID:          1,
				Username:    "testuser",
				Password:    "",
				MailAddress: "test@example.com",
			},
			skipPassword: false,
			wantErr:      true,
		},
		{
			name: "empty password when skipping",
			user: User{
				ID:          1,
				Username:    "testuser",
				MailAddress: "test@example.com",
			},
			skipPassword: true,
			wantErr:      false,
		},
		{
			name: "invalid email - no @",
			user: User{
				ID:          1,
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "invalidemail.com",
			},
			skipPassword: false,
			wantErr:      true,
		},
		{
			name: "invalid email - no domain",
			user: User{
				ID:          1,
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "test@",
			},
			skipPassword: false,
			wantErr:      true,
		},
		{
			name: "empty email",
			user: User{
				ID:          1,
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "",
			},
			skipPassword: false,
			wantErr:      true,
		},
		{
			name: "valid email with subdomain",
			user: User{
				ID:          1,
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "user@mail.example.com",
			},
			skipPassword: false,
			wantErr:      false,
		},
		{
			name: "password exactly 8 characters",
			user: User{
				ID:          1,
				Username:    "testuser",
				Password:    "12345678",
				MailAddress: "test@example.com",
			},
			skipPassword: false,
			wantErr:      false,
		},
		{
			name: "large valid user ID",
			user: User{
				ID:          999999,
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "test@example.com",
			},
			skipPassword: false,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.user.IsValid(tt.skipPassword)
			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
