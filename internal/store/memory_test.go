package store

import (
	"fmt"
	"sync"
	"testing"
)

func TestMemoryStore_ConcurrentSave(t *testing.T) {
	store := NewMemoryStore()
	var wg sync.WaitGroup

	const count = 1000
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			n := Notification{
				Recipient: fmt.Sprintf("user%d@example.com", i),
				Subject:   "Test",
				Channel:   "console",
			}
			_, err := store.Save(n)
			if err != nil {
				t.Errorf("Save() error: %s", err)
			}
		}(i)
	}

	wg.Wait()

	all, err := store.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error: %s", err)
	}
	if len(all) != count {
		t.Errorf("len = %d, want %d", len(all), count)
	}
}
