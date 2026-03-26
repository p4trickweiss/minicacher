package cache

import (
	"sync"
	"testing"
)

func TestInMemoryCache_BasicOperations(t *testing.T) {
	c := New(Options{})

	// Test Set and Get
	c.Set("key1", "value1")
	if got, exists := c.Get("key1"); !exists || got != "value1" {
		t.Errorf("Get(key1) = (%q, %v), want (%q, true)", got, exists, "value1")
	}

	// Test Get non-existent key
	if got, exists := c.Get("nonexistent"); exists {
		t.Errorf("Get(nonexistent) = (%q, %v), want (empty, false)", got, exists)
	}

	// Test Exists
	if !c.Exists("key1") {
		t.Error("Exists(key1) = false, want true")
	}
	if c.Exists("nonexistent") {
		t.Error("Exists(nonexistent) = true, want false")
	}

	// Test Update
	c.Set("key1", "value2")
	if got, exists := c.Get("key1"); !exists || got != "value2" {
		t.Errorf("Get(key1) after update = (%q, %v), want (%q, true)", got, exists, "value2")
	}

	// Test Delete
	c.Delete("key1")
	if got, exists := c.Get("key1"); exists {
		t.Errorf("Get(key1) after delete = (%q, %v), want (empty, false)", got, exists)
	}
	if c.Exists("key1") {
		t.Error("Exists(key1) after delete = true, want false")
	}

	// Test Delete non-existent key (should not panic)
	c.Delete("nonexistent")
}

func TestInMemoryCache_Snapshot(t *testing.T) {
	c := New(Options{})

	// Populate cache
	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")

	// Take snapshot
	snapshot := c.Snapshot()

	// Verify snapshot contents
	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	if len(snapshot) != len(expected) {
		t.Fatalf("Snapshot has %d entries, want %d", len(snapshot), len(expected))
	}

	for k, v := range expected {
		if snapshot[k] != v {
			t.Errorf("Snapshot[%q] = %q, want %q", k, snapshot[k], v)
		}
	}

	// Verify snapshot isolation - changes after snapshot don't affect it
	c.Set("key1", "modified")
	c.Set("key4", "value4")
	c.Delete("key2")

	if snapshot["key1"] != "value1" {
		t.Errorf("Snapshot[key1] = %q, want %q (snapshot should be isolated)", snapshot["key1"], "value1")
	}
	if _, exists := snapshot["key4"]; exists {
		t.Errorf("Snapshot contains key4, but it was added after snapshot")
	}
	if snapshot["key2"] != "value2" {
		t.Errorf("Snapshot[key2] = %q, want %q (should still exist in snapshot)", snapshot["key2"], "value2")
	}
}

func TestInMemoryCache_Restore(t *testing.T) {
	c := New(Options{})

	// Populate cache
	c.Set("key1", "value1")
	c.Set("key2", "value2")

	// Restore with new data
	newData := map[string]string{
		"key3": "value3",
		"key4": "value4",
	}
	c.Restore(newData)

	// Verify old keys are gone
	if _, exists := c.Get("key1"); exists {
		t.Error("Get(key1) after restore should not exist")
	}
	if _, exists := c.Get("key2"); exists {
		t.Error("Get(key2) after restore should not exist")
	}

	// Verify new keys exist
	if got, exists := c.Get("key3"); !exists || got != "value3" {
		t.Errorf("Get(key3) after restore = (%q, %v), want (%q, true)", got, exists, "value3")
	}
	if got, exists := c.Get("key4"); !exists || got != "value4" {
		t.Errorf("Get(key4) after restore = (%q, %v), want (%q, true)", got, exists, "value4")
	}
}

func TestInMemoryCache_ConcurrentReads(t *testing.T) {
	c := New(Options{})

	// Populate cache
	for i := 0; i < 100; i++ {
		c.Set(string(rune('a'+i%26)), "value")
	}

	// Concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				key := string(rune('a' + j%26))
				c.Get(key)
			}
		}()
	}

	wg.Wait()
}

func TestInMemoryCache_ConcurrentWrites(t *testing.T) {
	c := New(Options{})

	// Concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := string(rune('a' + j%26))
				c.Set(key, "value")
			}
		}(i)
	}

	wg.Wait()

	// Verify cache is in valid state
	for i := 0; i < 26; i++ {
		key := string(rune('a' + i))
		if got, exists := c.Get(key); !exists || got != "value" {
			t.Errorf("Get(%q) = (%q, %v), want (%q, true)", key, got, exists, "value")
		}
	}
}

func TestInMemoryCache_ConcurrentReadWrite(t *testing.T) {
	c := New(Options{})

	// Initialize some data
	for i := 0; i < 26; i++ {
		c.Set(string(rune('a'+i)), "initial")
	}

	// Concurrent readers and writers
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := string(rune('a' + j%26))
				c.Set(key, "updated")
			}
		}()
	}

	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := string(rune('a' + j%26))
				c.Get(key) // Just read, value can be either "initial" or "updated"
			}
		}()
	}

	// Deleters
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				key := string(rune('a' + j%26))
				c.Delete(key)
			}
		}()
	}

	wg.Wait()
}

func TestInMemoryCache_ConcurrentSnapshot(t *testing.T) {
	c := New(Options{})

	// Initialize data
	for i := 0; i < 10; i++ {
		c.Set(string(rune('a'+i)), "value")
	}

	// Concurrent snapshots and writes
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := string(rune('a' + j%10))
				c.Set(key, "updated")
			}
		}()
	}

	// Snapshotters
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Snapshot()
			}
		}()
	}

	wg.Wait()
}
