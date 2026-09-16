package service

import (
	"context"
	"errors"
	"testing"
)

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(context.Context) error {
	return f.err
}

func TestHealthService_Check(t *testing.T) {
	h := NewHealthService(fakePinger{})
	if err := h.Check(context.Background()); err != nil {
		t.Errorf("Check() error: %s", err)
	}

	wantErr := errors.New("connection refused")
	h = NewHealthService(fakePinger{err: wantErr})
	if err := h.Check(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("Check() error = %v, want %v", err, wantErr)
	}
}
