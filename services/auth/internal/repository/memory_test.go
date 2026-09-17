package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
	"uuid"

	"authservice/internal/domain"
)

func TestCreate(t *testing.T) {
	tests := []struct {
		name    string
		user    domain.User
		wantErr bool
	}{
		{
			name: "valid new user",
			user: domain.User{
				ID:           uuid.New(),
				Email:        "test@mail.com",
				PasswordHash: "some_hash",
				CreatedAt:    time.Now(),
			},
			wantErr: false,
		},
		{
			name: "exist user",
			user: domain.User{
				ID:           uuid.New(),
				Email:        "exist@mail.com",
				PasswordHash: "some_hash",
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMemoryRepository()

			if tt.wantErr {
				repo.users[tt.user.Email] = tt.user
			}

			id, err := repo.Create(context.Background(), tt.user)

			if tt.wantErr && err == nil {
				t.Errorf("want err, but got nil")
			}
			if tt.wantErr && err != nil {
				if !errors.Is(err, domain.ErrUserExists) {
					t.Errorf("want %s, but got: %s", domain.ErrUserExists, err)
				}
			}

			if !tt.wantErr && err == nil {
				if id == tt.user.ID {
					t.Errorf("id has no update, stub: %v == got: %v", tt.user.ID, id)
				}
				u := repo.users[tt.user.Email]
				if u.PasswordHash != tt.user.PasswordHash {
					t.Errorf("wrong password, want: %s, got: %s", tt.user.PasswordHash, u.PasswordHash)
				}
			}
			if !tt.wantErr && err != nil {
				t.Errorf("dont want err, but got: %s", err)
			}
		})
	}
}

func TestMemoryRepository_ConcurrentCreate(t *testing.T) {
	repo := NewMemoryRepository()
	var wg sync.WaitGroup

	const count = 1000
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			u := domain.User{
				ID:           uuid.New(),
				Email:        fmt.Sprintf("user%d@example.com", i),
				PasswordHash: fmt.Sprintf("test%d", i),
				CreatedAt:    time.Now(),
			}
			_, err := repo.Create(context.Background(), u)
			if err != nil {
				t.Errorf("Create() error: %s", err)
			}
		}(i)
	}

	wg.Wait()

	all := len(repo.users)
	if all != count {
		t.Errorf("len = %d, want %d", all, count)
	}
}

func TestGetByEmail(t *testing.T) {
	tests := []struct {
		name    string
		user    domain.User
		wantErr bool
	}{
		{
			name: "exist user",
			user: domain.User{
				ID:           uuid.New(),
				Email:        "exist@mail.com",
				PasswordHash: "some_hash",
				CreatedAt:    time.Now(),
			},
			wantErr: false,
		},
		{
			name: "no such user",
			user: domain.User{
				ID:           uuid.New(),
				Email:        "such@mail.com",
				PasswordHash: "some_hash",
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMemoryRepository()

			if !tt.wantErr {
				repo.users[tt.user.Email] = tt.user
			}

			u, err := repo.GetByEmail(context.Background(), tt.user.Email)

			if tt.wantErr && err == nil {
				t.Errorf("want err, but got nil")
			}
			if tt.wantErr && err != nil {
				if !errors.Is(err, domain.ErrInvalidCredentials) {
					t.Errorf("want %s, but got: %s", domain.ErrInvalidCredentials, err)
				}
			}

			if !tt.wantErr && err == nil {
				if u.ID != tt.user.ID {
					t.Errorf("wrong id, want: %v, got: %v", tt.user.ID, u.ID)
				}
				if u.PasswordHash != tt.user.PasswordHash {
					t.Errorf("wrong password, want: %s, got: %s", tt.user.PasswordHash, u.PasswordHash)
				}
			}
			if !tt.wantErr && err != nil {
				t.Errorf("dont want err, but got: %s", err)
			}
		})
	}
}
