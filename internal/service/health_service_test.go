package service

import (
	"context"
	"errors"
	"testing"
)

type fakeDbPinger struct {
	err error
}

type fakeCachePinger struct {
	err error
}

func (f fakeDbPinger) Ping(context.Context) error {
	return f.err
}

func (f fakeCachePinger) Ping(context.Context) error {
	return f.err
}

func TestHealthService_CheckDB(t *testing.T) {
	h := NewHealthService(
		fakeDbPinger{},
		fakeCachePinger{},
	)
	if err := h.CheckDB(context.Background()); err != nil {
		t.Errorf("CheckDB() error: %s", err)
	}

	wantErr := errors.New("connection refused")
	h = NewHealthService(
		fakeDbPinger{err: wantErr},
		fakeCachePinger{},
	)
	if err := h.CheckDB(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("CheckDB() error = %v, want %v", err, wantErr)
	}
}

func TestHealthService_CheckCache(t *testing.T) {
	h := NewHealthService(
		fakeDbPinger{},
		fakeCachePinger{},
	)
	if err := h.CheckCache(context.Background()); err != nil {
		t.Errorf("CheckCache() error: %s", err)
	}

	wantErr := errors.New("connection refused")
	h = NewHealthService(
		fakeDbPinger{},
		fakeCachePinger{err: wantErr},
	)
	if err := h.CheckCache(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("CheckCache() error = %v, want %v", err, wantErr)
	}
}
