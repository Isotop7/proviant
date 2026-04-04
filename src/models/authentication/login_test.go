package authentication

import "testing"

func TestLoginIsValid(t *testing.T) {
	tests := []struct {
		name    string
		login   Login
		wantErr bool
	}{
		{
			name: "valid login",
			login: Login{
				Username: "testuser",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "valid login with longer password",
			login: Login{
				Username: "testuser",
				Password: "verylongpassword123",
			},
			wantErr: false,
		},
		{
			name: "empty username",
			login: Login{
				Username: "",
				Password: "password123",
			},
			wantErr: true,
		},
		{
			name: "whitespace only username passes validation",
			login: Login{
				Username: "   ",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "password too short",
			login: Login{
				Username: "testuser",
				Password: "short",
			},
			wantErr: true,
		},
		{
			name: "password exactly 8 characters",
			login: Login{
				Username: "testuser",
				Password: "12345678",
			},
			wantErr: false,
		},
		{
			name: "password 7 characters",
			login: Login{
				Username: "testuser",
				Password: "1234567",
			},
			wantErr: true,
		},
		{
			name: "password 1 character",
			login: Login{
				Username: "testuser",
				Password: "1",
			},
			wantErr: true,
		},
		{
			name: "empty password",
			login: Login{
				Username: "testuser",
				Password: "",
			},
			wantErr: true,
		},
		{
			name: "all fields empty",
			login: Login{
				Username: "",
				Password: "",
			},
			wantErr: true,
		},
		{
			name: "username with special characters",
			login: Login{
				Username: "user-name_123",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "password with special characters",
			login: Login{
				Username: "testuser",
				Password: "p@ssw0rd!",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.login.IsValid()
			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
