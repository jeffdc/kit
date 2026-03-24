package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"mull/internal/model"
)

// CreateSession writes a new session file with the current timestamp.
func (s *Store) CreateSession(matters []string, body string) (*model.Session, error) {
	return s.CreateSessionAt(matters, body, time.Now())
}

// CreateSessionAt writes a new session file with the given timestamp.
func (s *Store) CreateSessionAt(matters []string, body string, ts time.Time) (*model.Session, error) {
	sess := &model.Session{
		Date:    ts,
		Matters: matters,
		Body:    body,
	}

	filename := ts.Format("2006-01-02T15-04") + ".md"
	sess.Filename = filename

	if err := s.writeSession(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// GetSession reads a session by its filename.
func (s *Store) GetSession(filename string) (*model.Session, error) {
	path := filepath.Join(s.sessionsDir, filename)
	return s.readSessionFile(path)
}

// ListSessions returns all sessions, most recent first.
// If matterID is non-empty, only sessions referencing that matter are returned.
func (s *Store) ListSessions(matterID string) ([]*model.Session, error) {
	entries, err := os.ReadDir(s.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []*model.Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		sess, err := s.readSessionFile(filepath.Join(s.sessionsDir, e.Name()))
		if err != nil {
			continue
		}
		if matterID != "" && !slices.Contains(sess.Matters, matterID) {
			continue
		}
		sessions = append(sessions, sess)
	}

	// Sort most recent first (filenames are timestamps, so reverse alpha works)
	slices.SortFunc(sessions, func(a, b *model.Session) int {
		return b.Date.Compare(a.Date)
	})

	return sessions, nil
}

// SessionContext returns the last n sessions, optionally filtered by matter.
func (s *Store) SessionContext(n int, matterID string) ([]*model.Session, error) {
	sessions, err := s.ListSessions(matterID)
	if err != nil {
		return nil, err
	}
	if len(sessions) > n {
		sessions = sessions[:n]
	}
	return sessions, nil
}

func (s *Store) writeSession(sess *model.Session) error {
	fm := yaml.Node{Kind: yaml.MappingNode}
	fm.Content = append(fm.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "date"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: sess.Date.Format("2006-01-02T15:04")},
	)
	if len(sess.Matters) > 0 {
		fm.Content = append(fm.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "matters"},
			strSliceNode(sess.Matters),
		)
	}

	fmBytes, err := yaml.Marshal(&fm)
	if err != nil {
		return err
	}

	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(fmBytes)
	buf.WriteString("---\n\n")
	buf.WriteString(sess.Body + "\n")

	path := filepath.Join(s.sessionsDir, sess.Filename)
	return os.WriteFile(path, []byte(buf.String()), 0644)
}

func (s *Store) readSessionFile(path string) (*model.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	sess := &model.Session{
		Filename: filepath.Base(path),
	}

	content := string(data)
	if strings.HasPrefix(content, "---\n") {
		parts := strings.SplitN(content[4:], "\n---\n", 2)
		if len(parts) == 2 {
			var raw struct {
				Date    string   `yaml:"date"`
				Matters []string `yaml:"matters"`
			}
			if err := yaml.Unmarshal([]byte(parts[0]), &raw); err != nil {
				return nil, fmt.Errorf("parsing session frontmatter %s: %w", sess.Filename, err)
			}
			t, err := time.Parse("2006-01-02T15:04", raw.Date)
			if err != nil {
				return nil, fmt.Errorf("parsing session date %s: %w", raw.Date, err)
			}
			sess.Date = t
			sess.Matters = raw.Matters
			sess.Body = strings.TrimSpace(parts[1])
		}
	}

	return sess, nil
}
