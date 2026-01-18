package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"watchmen/internal/model"
)

const CurrentVersion = 2

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrEntryNotFound   = errors.New("entry not found")
	ErrNoActiveEntry   = errors.New("no active time entry")
	ErrActiveEntry     = errors.New("there is already an active time entry")
	ErrNoPausedEntry   = errors.New("no paused time entry")
	ErrNotPaused       = errors.New("entry is not paused")
	ErrAlreadyPaused   = errors.New("entry is already paused")
)

// Store manages the JSON data file
type Store struct {
	path string
	data model.Data
}

// New creates a new Store, loading existing data if present
func New(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// DefaultPath returns the default data file path
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".watchmen")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "data.json"), nil
}

// v1Entry represents the old entry format for migration
type v1Entry struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id"`
	Note      string     `json:"note,omitempty"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}

// v1Data represents the old data format for migration
type v1Data struct {
	Version  int             `json:"version"`
	Projects []model.Project `json:"projects"`
	Entries  []v1Entry       `json:"entries"`
	Settings *model.Settings `json:"settings,omitempty"`
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		s.data = model.Data{Version: CurrentVersion}
		return nil
	}
	if err != nil {
		return err
	}

	// First, peek at the version
	var versionCheck struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &versionCheck); err != nil {
		return err
	}

	// If version is 0 or 1, migrate from old format
	if versionCheck.Version < CurrentVersion {
		return s.migrateFromV1(data)
	}

	return json.Unmarshal(data, &s.data)
}

func (s *Store) migrateFromV1(data []byte) error {
	var old v1Data
	if err := json.Unmarshal(data, &old); err != nil {
		return err
	}

	// Convert entries
	newEntries := make([]model.Entry, len(old.Entries))
	for i, e := range old.Entries {
		seg := model.TimeSegment{
			Start: e.StartTime,
			End:   e.EndTime,
		}
		newEntries[i] = model.Entry{
			ID:        e.ID,
			ProjectID: e.ProjectID,
			Note:      e.Note,
			Segments:  []model.TimeSegment{seg},
			Completed: e.EndTime != nil,
		}
	}

	s.data = model.Data{
		Version:  CurrentVersion,
		Projects: old.Projects,
		Entries:  newEntries,
		Settings: old.Settings,
	}

	// Save migrated data
	return s.save()
}

func (s *Store) save() error {
	s.data.Version = CurrentVersion
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// AddProject creates a new project
func (s *Store) AddProject(name string, hourlyRate float64, description string) (*model.Project, error) {
	p := model.Project{
		ID:          generateID(),
		Name:        name,
		HourlyRate:  hourlyRate,
		Description: description,
		CreatedAt:   time.Now(),
	}
	s.data.Projects = append(s.data.Projects, p)
	return &p, s.save()
}

// GetProject returns a project by ID or name
func (s *Store) GetProject(idOrName string) (*model.Project, error) {
	for i := range s.data.Projects {
		if s.data.Projects[i].ID == idOrName || s.data.Projects[i].Name == idOrName {
			return &s.data.Projects[i], nil
		}
	}
	return nil, ErrProjectNotFound
}

// ListProjects returns all projects
func (s *Store) ListProjects() []model.Project {
	return s.data.Projects
}

// StartEntry starts a new time entry
func (s *Store) StartEntry(projectID, note string) (*model.Entry, error) {
	// Check for existing active or paused entry
	for _, e := range s.data.Entries {
		if e.IsRunning() || e.IsPaused() {
			return nil, ErrActiveEntry
		}
	}

	// Verify project exists
	if _, err := s.GetProject(projectID); err != nil {
		return nil, err
	}

	now := time.Now()
	entry := model.Entry{
		ID:        generateID(),
		ProjectID: projectID,
		Note:      note,
		Segments: []model.TimeSegment{
			{Start: now},
		},
		Completed: false,
	}
	s.data.Entries = append(s.data.Entries, entry)
	return &entry, s.save()
}

// StopEntry stops the current active or paused entry
func (s *Store) StopEntry(note string) (*model.Entry, error) {
	for i := range s.data.Entries {
		if s.data.Entries[i].IsRunning() || s.data.Entries[i].IsPaused() {
			// If running, close the current segment
			if s.data.Entries[i].IsRunning() {
				now := time.Now()
				lastIdx := len(s.data.Entries[i].Segments) - 1
				s.data.Entries[i].Segments[lastIdx].End = &now
			}

			s.data.Entries[i].Completed = true

			if note != "" {
				if s.data.Entries[i].Note != "" {
					s.data.Entries[i].Note += " | " + note
				} else {
					s.data.Entries[i].Note = note
				}
			}
			if err := s.save(); err != nil {
				return nil, err
			}
			return &s.data.Entries[i], nil
		}
	}
	return nil, ErrNoActiveEntry
}

// PauseEntry pauses the current running entry
func (s *Store) PauseEntry() (*model.Entry, error) {
	for i := range s.data.Entries {
		if s.data.Entries[i].IsRunning() {
			now := time.Now()
			lastIdx := len(s.data.Entries[i].Segments) - 1
			s.data.Entries[i].Segments[lastIdx].End = &now

			if err := s.save(); err != nil {
				return nil, err
			}
			return &s.data.Entries[i], nil
		}
		if s.data.Entries[i].IsPaused() {
			return nil, ErrAlreadyPaused
		}
	}
	return nil, ErrNoActiveEntry
}

// ResumeEntry resumes a paused entry
func (s *Store) ResumeEntry() (*model.Entry, error) {
	for i := range s.data.Entries {
		if s.data.Entries[i].IsPaused() {
			now := time.Now()
			s.data.Entries[i].Segments = append(s.data.Entries[i].Segments, model.TimeSegment{Start: now})

			if err := s.save(); err != nil {
				return nil, err
			}
			return &s.data.Entries[i], nil
		}
		if s.data.Entries[i].IsRunning() {
			return nil, ErrActiveEntry
		}
	}
	return nil, ErrNoPausedEntry
}

// LogEntry creates a completed time entry
func (s *Store) LogEntry(projectID, note string, start, end time.Time) (*model.Entry, error) {
	if _, err := s.GetProject(projectID); err != nil {
		return nil, err
	}

	entry := model.Entry{
		ID:        generateID(),
		ProjectID: projectID,
		Note:      note,
		Segments: []model.TimeSegment{
			{Start: start, End: &end},
		},
		Completed: true,
	}
	s.data.Entries = append(s.data.Entries, entry)
	return &entry, s.save()
}

// ActiveEntry returns the current running or paused entry, if any
func (s *Store) ActiveEntry() *model.Entry {
	for i := range s.data.Entries {
		if s.data.Entries[i].IsRunning() || s.data.Entries[i].IsPaused() {
			return &s.data.Entries[i]
		}
	}
	return nil
}

// ListEntries returns entries, optionally filtered by project and date range
func (s *Store) ListEntries(projectID string, from, to *time.Time) []model.Entry {
	var result []model.Entry
	for _, e := range s.data.Entries {
		if projectID != "" && e.ProjectID != projectID {
			// Check if project name matches
			p, _ := s.GetProject(projectID)
			if p == nil || p.ID != e.ProjectID {
				continue
			}
		}
		startTime := e.StartTime()
		if from != nil && startTime.Before(*from) {
			continue
		}
		if to != nil && startTime.After(*to) {
			continue
		}
		result = append(result, e)
	}
	return result
}

// DeleteEntry removes an entry by ID
func (s *Store) DeleteEntry(id string) error {
	for i, e := range s.data.Entries {
		if e.ID == id {
			s.data.Entries = append(s.data.Entries[:i], s.data.Entries[i+1:]...)
			return s.save()
		}
	}
	return ErrEntryNotFound
}

// GetSettings returns the current settings
func (s *Store) GetSettings() *model.Settings {
	if s.data.Settings == nil {
		return &model.Settings{}
	}
	return s.data.Settings
}

// SetUserContact updates the user's contact info
func (s *Store) SetUserContact(contact *model.ContactInfo) error {
	if s.data.Settings == nil {
		s.data.Settings = &model.Settings{}
	}
	s.data.Settings.UserContact = contact
	return s.save()
}

// UpdateProject updates a project's fields
func (s *Store) UpdateProject(idOrName string, updates func(*model.Project)) error {
	for i := range s.data.Projects {
		if s.data.Projects[i].ID == idOrName || s.data.Projects[i].Name == idOrName {
			updates(&s.data.Projects[i])
			return s.save()
		}
	}
	return ErrProjectNotFound
}
