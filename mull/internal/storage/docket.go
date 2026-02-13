package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"mull/internal/model"
)

func (s *Store) docketPath() string {
	return filepath.Join(s.root, "docket.yml")
}

// LoadDocket reads .mull/docket.yml. Returns empty slice if file doesn't exist.
func (s *Store) LoadDocket() ([]model.DocketEntry, error) {
	data, err := os.ReadFile(s.docketPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []model.DocketEntry{}, nil
		}
		return nil, err
	}

	var entries []model.DocketEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing docket.yml: %w", err)
	}
	if entries == nil {
		entries = []model.DocketEntry{}
	}
	return entries, nil
}

// SaveDocket writes entries to .mull/docket.yml.
func (s *Store) SaveDocket(entries []model.DocketEntry) error {
	data, err := yaml.Marshal(entries)
	if err != nil {
		return err
	}
	return os.WriteFile(s.docketPath(), data, 0644)
}

// docketIndex returns the index of an entry by ID, or -1 if not found.
func docketIndex(entries []model.DocketEntry, id string) int {
	for i, e := range entries {
		if e.ID == id {
			return i
		}
	}
	return -1
}

// DocketAdd adds an entry. If afterID is non-empty, the new entry is inserted
// after the entry with that ID; otherwise it is appended to the end.
func (s *Store) DocketAdd(id string, afterID string, note string) error {
	entries, err := s.LoadDocket()
	if err != nil {
		return err
	}

	if docketIndex(entries, id) >= 0 {
		return fmt.Errorf("already in docket: %s", id)
	}

	entry := model.DocketEntry{ID: id, Note: note}

	if afterID == "" {
		entries = append(entries, entry)
	} else {
		idx := docketIndex(entries, afterID)
		if idx < 0 {
			return fmt.Errorf("after-id not found in docket: %s", afterID)
		}
		// Insert after idx
		entries = append(entries, model.DocketEntry{})
		copy(entries[idx+2:], entries[idx+1:])
		entries[idx+1] = entry
	}

	return s.SaveDocket(entries)
}

// DocketRemove removes an entry by ID.
func (s *Store) DocketRemove(id string) error {
	entries, err := s.LoadDocket()
	if err != nil {
		return err
	}

	idx := docketIndex(entries, id)
	if idx < 0 {
		return fmt.Errorf("not in docket: %s", id)
	}

	entries = append(entries[:idx], entries[idx+1:]...)
	return s.SaveDocket(entries)
}

// DocketMove moves an entry to after another ID.
func (s *Store) DocketMove(id string, afterID string) error {
	entries, err := s.LoadDocket()
	if err != nil {
		return err
	}

	idx := docketIndex(entries, id)
	if idx < 0 {
		return fmt.Errorf("not in docket: %s", id)
	}

	// Remove the entry
	entry := entries[idx]
	entries = append(entries[:idx], entries[idx+1:]...)

	// Find insertion point
	afterIdx := docketIndex(entries, afterID)
	if afterIdx < 0 {
		return fmt.Errorf("after-id not found in docket: %s", afterID)
	}

	// Insert after afterIdx
	entries = append(entries, model.DocketEntry{})
	copy(entries[afterIdx+2:], entries[afterIdx+1:])
	entries[afterIdx+1] = entry

	return s.SaveDocket(entries)
}
