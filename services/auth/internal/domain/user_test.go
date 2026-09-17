package domain

import (
	"testing"
	"time"
	"uuid"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		u       User
		wantErr bool
	}{
		{
			name: "valid user",
			u: User{
				ID:           uuid.New(),
				Email:        "test@mail.com",
				PasswordHash: "some_hash",
				CreatedAt:    time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty id",
			u: User{
				Email:        "test@mail.com",
				PasswordHash: "some_hash",
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty email",
			u: User{
				ID:           uuid.New(),
				Email:        "",
				PasswordHash: "some_hash",
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
		{
			name: "wrong email",
			u: User{
				ID:           uuid.New(),
				Email:        "test @mail.com",
				PasswordHash: "some_hash",
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty password",
			u: User{
				ID:           uuid.New(),
				Email:        "test@mail.com",
				PasswordHash: "",
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
		{
			name: "long password",
			u: User{
				ID:           uuid.New(),
				Email:        "test@mail.com",
				PasswordHash: string(make([]byte, 61)),
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
		{
			name: "zero time",
			u: User{
				ID:           uuid.New(),
				Email:        "test@mail.com",
				PasswordHash: "some_hash",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.u.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error: %s", err)
			}
		})
	}
}
