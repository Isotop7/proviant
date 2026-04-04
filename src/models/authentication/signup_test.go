package authentication

import "testing"

func TestSignupIsValid(t *testing.T) {
	tests := []struct {
		name    string
		signup  Signup
		wantErr bool
	}{
		{
			name: "valid signup",
			signup: Signup{
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "test@example.com",
			},
			wantErr: false,
		},
		{
			name: "valid signup with longer password",
			signup: Signup{
				Username:    "testuser",
				Password:    "verylongpassword123",
				MailAddress: "test@example.com",
			},
			wantErr: false,
		},
		{
			name: "empty username",
			signup: Signup{
				Username:    "",
				Password:    "password123",
				MailAddress: "test@example.com",
			},
			wantErr: true,
		},
		{
			name: "password too short",
			signup: Signup{
				Username:    "testuser",
				Password:    "short",
				MailAddress: "test@example.com",
			},
			wantErr: true,
		},
		{
			name: "password exactly 8 characters",
			signup: Signup{
				Username:    "testuser",
				Password:    "12345678",
				MailAddress: "test@example.com",
			},
			wantErr: false,
		},
		{
			name: "password 7 characters",
			signup: Signup{
				Username:    "testuser",
				Password:    "1234567",
				MailAddress: "test@example.com",
			},
			wantErr: true,
		},
		{
			name: "invalid email - no @",
			signup: Signup{
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "invalidemail.com",
			},
			wantErr: true,
		},
		{
			name: "invalid email - no domain",
			signup: Signup{
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "test@",
			},
			wantErr: true,
		},
		{
			name: "invalid email - no local part",
			signup: Signup{
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "@example.com",
			},
			wantErr: true,
		},
		{
			name: "empty email",
			signup: Signup{
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "",
			},
			wantErr: true,
		},
		{
			name: "valid email with subdomain",
			signup: Signup{
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "user@mail.example.com",
			},
			wantErr: false,
		},
		{
			name: "valid email with plus sign",
			signup: Signup{
				Username:    "testuser",
				Password:    "password123",
				MailAddress: "user+tag@example.com",
			},
			wantErr: false,
		},
		{
			name: "all fields empty",
			signup: Signup{
				Username:    "",
				Password:    "",
				MailAddress: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.signup.IsValid()
			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
