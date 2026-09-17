package service

import (
	"context"
	"errors"
	"testing"
)

type fakeDbPinger struct {
	err error
}

func (f fakeDbPinger) Ping(context.Context) error {
	return f.err
}

func TestHealthService_CheckDB(t *testing.T) {
	h := NewHealthService(
		fakeDbPinger{},
	)
	if err := h.CheckDB(context.Background()); err != nil {
		t.Errorf("CheckDB() error: %s", err)
	}

	wantErr := errors.New("connection refused")
	h = NewHealthService(
		fakeDbPinger{err: wantErr},
	)
	if err := h.CheckDB(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("CheckDB() error = %v, want %v", err, wantErr)
	}
}
