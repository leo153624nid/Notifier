package store

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"notifier/internal/notification"
)

func TestMemoryStore_ConcurrentSave(t *testing.T) {
	store := NewMemoryStore()
	var wg sync.WaitGroup

	const count = 1000
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			n := notification.Notification{
				Recipient: fmt.Sprintf("user%d@example.com", i),
				Subject:   "Test",
				Channel:   "console",
			}
			_, err := store.Save(context.Background(), n)
			if err != nil {
				t.Errorf("Save() error: %s", err)
			}
		}(i)
	}

	wg.Wait()

	all, err := store.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll() error: %s", err)
	}
	if len(all) != count {
		t.Errorf("len = %d, want %d", len(all), count)
	}
}

func TestGetList(t *testing.T) {
	all := []notification.Notification{
		{ID: 12},
		{ID: 11},
		{ID: 10},
		{ID: 9},
		{ID: 8},
		{ID: 7},
		{ID: 6},
		{ID: 5},
		{ID: 4},
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}
	tests := []struct {
		name    string
		all     []notification.Notification
		page    int
		size    int
		firstID int
		lastID  int
		len     int
	}{
		{
			name:    "full page",
			all:     all,
			page:    1,
			size:    2,
			firstID: 1,
			lastID:  2,
			len:     2,
		},
		{
			name:    "not full page",
			all:     all,
			page:    2,
			size:    10,
			firstID: 11,
			lastID:  12,
			len:     2,
		},
		{
			name:    "zero page",
			all:     all,
			page:    0,
			size:    2,
			firstID: 1,
			lastID:  2,
			len:     2,
		},
		{
			name:    "invalid page",
			all:     all,
			page:    -1,
			size:    2,
			firstID: 1,
			lastID:  2,
			len:     2,
		},
		{
			name:    "zero size",
			all:     all,
			page:    1,
			size:    0,
			firstID: 1,
			lastID:  10,
			len:     10,
		},
		{
			name:    "invalid size",
			all:     all,
			page:    1,
			size:    -1,
			firstID: 1,
			lastID:  10,
			len:     10,
		},
		{
			name:    "invalid page && size",
			all:     all,
			page:    -1,
			size:    -2,
			firstID: 1,
			lastID:  10,
			len:     10,
		},
		{
			name:    "over len",
			all:     all,
			page:    3,
			size:    10,
			firstID: 0,
			lastID:  0,
			len:     0,
		},
		{
			name:    "over size",
			all:     all,
			page:    1,
			size:    1000,
			firstID: 1,
			lastID:  12,
			len:     12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMemoryStore()
			for _, n := range tt.all {
				store.notifications[n.ID] = n
			}

			list, _ := store.GetList(context.Background(), tt.page, tt.size)

			if len(list) != tt.len {
				t.Errorf("list len %d, want %d", len(list), tt.len)
			}
			if len(list) > 0 {
				if firstID := list[0].ID; firstID != tt.firstID {
					t.Errorf("first ID: %d, want: %d", firstID, tt.firstID)
				}
				if lastID := list[len(list)-1].ID; lastID != tt.lastID {
					t.Errorf("last ID: %d, want: %d", lastID, tt.lastID)
				}
			}
		})
	}
}
