package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"hearthlane-relay/internal/state"
)

type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: path}
}

func (st *Store) Load() (*state.State, error) {
	data, err := os.ReadFile(st.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return state.New(), nil
		}
		return nil, fmt.Errorf("read state file %q: %w", st.path, err)
	}
	s := state.New()
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("state file %q is invalid: %w", st.path, err)
	}
	if s.Devices == nil {
		s.Devices = make(map[string]*state.Device)
	}
	if err := validate(s); err != nil {
		return nil, fmt.Errorf("state file %q is invalid: %w", st.path, err)
	}
	return s, nil
}

func validate(s *state.State) error {
	for id, dev := range s.Devices {
		if dev == nil {
			return fmt.Errorf("device %q is null", id)
		}
		if dev.Location == nil {
			continue
		}
		if err := state.ValidateCoordinates(dev.Location.Latitude, dev.Location.Longitude); err != nil {
			return fmt.Errorf("device %q: %w", id, err)
		}
		if dev.Location.PublishedAtEpochMs <= 0 {
			return fmt.Errorf("device %q: publishedAtEpochMs must be positive", id)
		}
	}
	return nil
}

func (st *Store) Save(data []byte) error {
	dir := filepath.Dir(st.path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(st.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	removeTmp := func() {
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		removeTmp()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		removeTmp()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		removeTmp()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, st.path); err != nil {
		removeTmp()
		return fmt.Errorf("atomic rename to state file: %w", err)
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
