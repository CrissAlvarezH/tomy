package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Buckets
var allBuckets = []string{
	"projects",
	"tasks",
	"plans",
	"plan_content",
	"workers",
	"inbox",
	"nudges",
	"meta",
}

// DB wraps a bbolt database.
type DB struct {
	bolt *bolt.DB
}

// Open opens (or creates) the bbolt database at path and ensures all buckets exist.
func Open(path string) (*DB, error) {
	bdb, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Create all buckets on first run
	err = bdb.Update(func(tx *bolt.Tx) error {
		for _, name := range allBuckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bucket %q: %w", name, err)
			}
		}
		return nil
	})
	if err != nil {
		bdb.Close()
		return nil, err
	}

	return &DB{bolt: bdb}, nil
}

// OpenReadOnly opens the bbolt database in read-only mode.
// Multiple read-only opens can coexist with a read-write open.
func OpenReadOnly(path string) (*DB, error) {
	bdb, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("open database (read-only): %w", err)
	}
	return &DB{bolt: bdb}, nil
}

// Close closes the database.
func (db *DB) Close() error {
	return db.bolt.Close()
}

// Bolt returns the raw *bolt.DB for direct transaction access.
func (db *DB) Bolt() *bolt.DB {
	return db.bolt
}

// Get retrieves a single JSON-encoded value from bucket by key.
// Returns an error if the key doesn't exist.
func (db *DB) Get(bucket, key string, dest any) error {
	return db.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("key %q not found in %q", key, bucket)
		}
		return json.Unmarshal(v, dest)
	})
}

// GetRaw retrieves raw bytes from bucket by key. Returns nil if not found.
func (db *DB) GetRaw(bucket, key string) ([]byte, error) {
	var result []byte
	err := db.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(key))
		if v != nil {
			result = make([]byte, len(v))
			copy(result, v)
		}
		return nil
	})
	return result, err
}

// Put JSON-encodes value and stores it in bucket under key.
func (db *DB) Put(bucket, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Put([]byte(key), data)
	})
}

// PutRaw stores raw bytes in bucket under key.
func (db *DB) PutRaw(bucket, key string, data []byte) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Put([]byte(key), data)
	})
}

// Delete removes a key from a bucket.
func (db *DB) Delete(bucket, key string) error {
	return db.bolt.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Delete([]byte(key))
	})
}

// List iterates all values in a bucket and returns them as a slice.
func List[T any](db *DB, bucket string) ([]T, error) {
	var items []T
	err := db.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.ForEach(func(k, v []byte) error {
			var item T
			if err := json.Unmarshal(v, &item); err != nil {
				return err
			}
			items = append(items, item)
			return nil
		})
	})
	return items, err
}

// ListByPrefix iterates keys with a given prefix, returning decoded values and their keys.
func ListByPrefix[T any](db *DB, bucket, prefix string) ([]T, []string, error) {
	var items []T
	var keys []string
	pfx := []byte(prefix)

	err := db.bolt.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(bucket)).Cursor()
		for k, v := c.Seek(pfx); k != nil && bytes.HasPrefix(k, pfx); k, v = c.Next() {
			var item T
			if err := json.Unmarshal(v, &item); err != nil {
				return err
			}
			items = append(items, item)
			keys = append(keys, string(k))
		}
		return nil
	})
	return items, keys, err
}

// DrainByPrefix reads and deletes all keys with a given prefix in a single transaction.
// Returns decoded values sorted by key (lexicographic).
func DrainByPrefix[T any](db *DB, bucket, prefix string) ([]T, error) {
	var items []T
	pfx := []byte(prefix)

	err := db.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()
		var toDelete [][]byte
		for k, v := c.Seek(pfx); k != nil && bytes.HasPrefix(k, pfx); k, v = c.Next() {
			var item T
			if err := json.Unmarshal(v, &item); err != nil {
				return err
			}
			items = append(items, item)
			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)
			toDelete = append(toDelete, keyCopy)
		}
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
	return items, err
}
