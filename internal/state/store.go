package state

import (
	"encoding/json"
	"os"
	"syscall"
)

// ReadJSON reads a JSON file into the given pointer, with file locking.
// If the file doesn't exist, it leaves target unchanged (caller should initialize defaults).
func ReadJSON(path string, target any) error {
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return nil // empty file, keep target as-is
	}

	return json.NewDecoder(f).Decode(target)
}

// WriteJSON writes data to a JSON file with exclusive file locking.
func WriteJSON(path string, data any) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
