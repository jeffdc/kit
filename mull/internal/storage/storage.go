package storage

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
	"mull/internal/model"
)

type Store struct {
	root       string // path to .mull/ directory
	mattersDir string
}

func New(dir string) (*Store, error) {
	root := filepath.Join(dir, ".mull")
	mattersDir := filepath.Join(root, "matters")

	if err := os.MkdirAll(mattersDir, 0755); err != nil {
		return nil, err
	}

	return &Store{root: root, mattersDir: mattersDir}, nil
}

func (s *Store) Root() string {
	return s.root
}

// GenerateID produces a 4-char hex ID from title + current time.
// If there's a collision with existing matters, it retries with incremented timestamps.
func (s *Store) GenerateID(title string) (string, error) {
	t := time.Now()
	for i := 0; i < 100; i++ {
		ts := t.Add(time.Duration(i) * time.Nanosecond).Format(time.RFC3339Nano)
		h := sha256.Sum256([]byte(title + ts))
		id := fmt.Sprintf("%x", h[:2])
		if _, err := s.findMatterFile(id); err != nil {
			return id, nil
		}
	}
	return "", fmt.Errorf("could not generate unique ID after 100 attempts")
}

// Slugify converts a title to a URL-friendly slug.
func Slugify(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, s)
	re := regexp.MustCompile(`-+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// CreateMatter creates a new matter file and returns the matter.
func (s *Store) CreateMatter(title string, meta map[string]any) (*model.Matter, error) {
	id, err := s.GenerateID(title)
	if err != nil {
		return nil, err
	}

	today := model.Today()
	m := &model.Matter{
		ID:      id,
		Title:   title,
		Status:  "raw",
		Created: today,
		Updated: today,
	}

	// Apply any provided metadata
	if meta != nil {
		if err := applyMeta(m, meta); err != nil {
			return nil, err
		}
	}

	filename := fmt.Sprintf("%s-%s.md", id, Slugify(title))
	m.Filename = filename

	if err := s.WriteMatter(m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetMatter reads a matter by its 4-char ID.
func (s *Store) GetMatter(id string) (*model.Matter, error) {
	path, err := s.findMatterFile(id)
	if err != nil {
		return nil, err
	}
	return s.readMatterFile(path)
}

// ListMatters returns all matters, optionally filtered.
func (s *Store) ListMatters(filters map[string]string) ([]*model.Matter, error) {
	entries, err := os.ReadDir(s.mattersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var matters []*model.Matter
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		m, err := s.readMatterFile(filepath.Join(s.mattersDir, e.Name()))
		if err != nil {
			continue
		}
		if matchesFilters(m, filters) {
			matters = append(matters, m)
		}
	}
	return matters, nil
}

// SearchMatters does full-text search across titles and bodies.
func (s *Store) SearchMatters(query string) ([]*model.Matter, error) {
	all, err := s.ListMatters(nil)
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	var results []*model.Matter
	for _, m := range all {
		if strings.Contains(strings.ToLower(m.Title), q) ||
			strings.Contains(strings.ToLower(m.Body), q) {
			results = append(results, m)
		}
	}
	return results, nil
}

// UpdateMatter sets metadata fields on a matter.
func (s *Store) UpdateMatter(id string, key string, value string) (*model.Matter, error) {
	m, err := s.GetMatter(id)
	if err != nil {
		return nil, err
	}

	if err := applyMeta(m, map[string]any{key: value}); err != nil {
		return nil, err
	}
	m.Updated = model.Today()

	if err := s.WriteMatter(m); err != nil {
		return nil, err
	}
	return m, nil
}

// AppendBody appends text to a matter's body.
func (s *Store) AppendBody(id string, text string) (*model.Matter, error) {
	m, err := s.GetMatter(id)
	if err != nil {
		return nil, err
	}

	if m.Body == "" {
		m.Body = text
	} else {
		m.Body = m.Body + "\n\n" + text
	}
	m.Updated = model.Today()

	if err := s.WriteMatter(m); err != nil {
		return nil, err
	}
	return m, nil
}

// DeleteMatter removes a matter file.
func (s *Store) DeleteMatter(id string) error {
	path, err := s.findMatterFile(id)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// findMatterFile locates a matter file by its ID prefix.
func (s *Store) findMatterFile(id string) (string, error) {
	entries, err := os.ReadDir(s.mattersDir)
	if err != nil {
		return "", fmt.Errorf("matter not found: %s", id)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), id+"-") && strings.HasSuffix(e.Name(), ".md") {
			return filepath.Join(s.mattersDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("matter not found: %s", id)
}

// readMatterFile parses a matter markdown file with YAML frontmatter.
func (s *Store) readMatterFile(path string) (*model.Matter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(path)
	id := strings.SplitN(filename, "-", 2)[0]

	m := &model.Matter{
		ID:       id,
		Filename: filename,
	}

	content := string(data)
	if strings.HasPrefix(content, "---\n") {
		parts := strings.SplitN(content[4:], "\n---\n", 2)
		if len(parts) == 2 {
			// Parse known fields
			if err := yaml.Unmarshal([]byte(parts[0]), m); err != nil {
				return nil, fmt.Errorf("parsing frontmatter of %s: %w", filename, err)
			}

			// Parse extra fields
			var raw map[string]any
			if err := yaml.Unmarshal([]byte(parts[0]), &raw); err == nil {
				m.Extra = extractExtra(raw)
			}

			body := strings.TrimSpace(parts[1])
			// Extract title from first heading
			if strings.HasPrefix(body, "# ") {
				lines := strings.SplitN(body, "\n", 2)
				m.Title = strings.TrimPrefix(lines[0], "# ")
				if len(lines) > 1 {
					m.Body = strings.TrimSpace(lines[1])
				}
			} else {
				m.Body = body
			}
		}
	}

	return m, nil
}

// WriteMatter writes a matter to its file.
func (s *Store) WriteMatter(m *model.Matter) error {
	// Build frontmatter map preserving known fields
	fm := buildFrontmatter(m)

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return err
	}

	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(fmBytes)
	buf.WriteString("---\n\n")
	buf.WriteString("# " + m.Title + "\n")
	if m.Body != "" {
		buf.WriteString("\n" + m.Body + "\n")
	}

	path := filepath.Join(s.mattersDir, m.Filename)
	return os.WriteFile(path, []byte(buf.String()), 0644)
}

// buildFrontmatter creates an ordered map for YAML serialization.
func buildFrontmatter(m *model.Matter) yaml.Node {
	doc := yaml.Node{Kind: yaml.MappingNode}

	addField := func(key, value string) {
		if value == "" {
			return
		}
		doc.Content = append(doc.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value},
		)
	}

	addStringSlice := func(key string, values []string) {
		if len(values) == 0 {
			return
		}
		doc.Content = append(doc.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			strSliceNode(values),
		)
	}

	addField("status", m.Status)
	addStringSlice("tags", m.Tags)
	addField("effort", m.Effort)
	addField("created", m.Created)
	addField("updated", m.Updated)
	addField("plan", m.Plan)
	addField("epic", m.Epic)

	// Relationships
	addStringSlice("relates", m.Relates)
	addStringSlice("blocks", m.Blocks)
	addStringSlice("needs", m.Needs)
	addField("parent", m.Parent)

	// Extra fields
	for k, v := range m.Extra {
		doc.Content = append(doc.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v)},
		)
	}

	return doc
}

func strSliceNode(values []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, v := range values {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
	}
	return seq
}

// knownFields are the YAML keys handled by struct tags.
var knownFields = map[string]bool{
	"status": true, "tags": true, "effort": true,
	"created": true, "updated": true, "plan": true, "epic": true,
	"relates": true, "blocks": true, "needs": true, "parent": true,
}

// extractExtra pulls out non-standard frontmatter fields.
func extractExtra(raw map[string]any) map[string]any {
	extra := make(map[string]any)
	for k, v := range raw {
		if !knownFields[k] {
			extra[k] = v
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

// applyMeta sets metadata fields on a matter from a map.
func applyMeta(m *model.Matter, meta map[string]any) error {
	for k, v := range meta {
		sv := fmt.Sprintf("%v", v)
		switch k {
		case "status":
			if err := model.ValidateStatus(sv); err != nil {
				return err
			}
			m.Status = sv
		case "effort":
			m.Effort = sv
		case "plan":
			m.Plan = sv
		case "epic":
			m.Epic = sv
		case "parent":
			m.Parent = sv
		case "tags":
			switch t := v.(type) {
			case []string:
				m.Tags = t
			case string:
				m.Tags = strings.Split(t, ",")
				for i := range m.Tags {
					m.Tags[i] = strings.TrimSpace(m.Tags[i])
				}
			}
		default:
			if m.Extra == nil {
				m.Extra = make(map[string]any)
			}
			m.Extra[k] = v
		}
	}
	return nil
}

// validRelTypes lists the allowed relationship types.
var validRelTypes = map[string]bool{
	"relates": true,
	"blocks":  true,
	"needs":   true,
	"parent":  true,
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// removeString returns a new slice with the first occurrence of s removed.
func removeString(slice []string, s string) []string {
	for i, v := range slice {
		if v == s {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// backupMatterFile reads the raw file content for rollback purposes.
func (s *Store) backupMatterFile(id string) (string, []byte, error) {
	path, err := s.findMatterFile(id)
	if err != nil {
		return "", nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	return path, data, nil
}

// LinkMatters creates a relationship between two matters.
// relType is one of: "relates", "blocks", "needs", "parent".
// The link is bidirectional (except parent which is one-way).
// If writing the second matter fails, the first is rolled back.
func (s *Store) LinkMatters(id1, relType, id2 string) error {
	if !validRelTypes[relType] {
		return fmt.Errorf("invalid relationship type: %s", relType)
	}

	m1, err := s.GetMatter(id1)
	if err != nil {
		return err
	}

	if relType == "parent" {
		// For parent, just verify id2 exists and set it.
		if _, err := s.GetMatter(id2); err != nil {
			return err
		}
		if m1.Parent == id2 {
			return nil // already set
		}
		m1.Parent = id2
		m1.Updated = model.Today()
		return s.WriteMatter(m1)
	}

	m2, err := s.GetMatter(id2)
	if err != nil {
		return err
	}

	// Determine what to add to each side.
	var needWriteM1, needWriteM2 bool

	switch relType {
	case "relates":
		if !containsString(m1.Relates, id2) {
			m1.Relates = append(m1.Relates, id2)
			needWriteM1 = true
		}
		if !containsString(m2.Relates, id1) {
			m2.Relates = append(m2.Relates, id1)
			needWriteM2 = true
		}
	case "blocks":
		if !containsString(m1.Blocks, id2) {
			m1.Blocks = append(m1.Blocks, id2)
			needWriteM1 = true
		}
		if !containsString(m2.Needs, id1) {
			m2.Needs = append(m2.Needs, id1)
			needWriteM2 = true
		}
	case "needs":
		if !containsString(m1.Needs, id2) {
			m1.Needs = append(m1.Needs, id2)
			needWriteM1 = true
		}
		if !containsString(m2.Blocks, id1) {
			m2.Blocks = append(m2.Blocks, id1)
			needWriteM2 = true
		}
	}

	if !needWriteM1 && !needWriteM2 {
		return nil // nothing to do
	}

	// Backup m1 before writing, for rollback.
	_, backupData, err := s.backupMatterFile(id1)
	if err != nil {
		return err
	}

	if needWriteM1 {
		m1.Updated = model.Today()
		if err := s.WriteMatter(m1); err != nil {
			return err
		}
	}

	if needWriteM2 {
		m2.Updated = model.Today()
		if err := s.WriteMatter(m2); err != nil {
			// Roll back m1.
			rollbackPath := filepath.Join(s.mattersDir, m1.Filename)
			os.WriteFile(rollbackPath, backupData, 0644)
			return fmt.Errorf("linking failed, rolled back %s: %w", id1, err)
		}
	}

	return nil
}

// UnlinkMatters removes a relationship between two matters.
// Reverse of LinkMatters with the same atomicity guarantee.
func (s *Store) UnlinkMatters(id1, relType, id2 string) error {
	if !validRelTypes[relType] {
		return fmt.Errorf("invalid relationship type: %s", relType)
	}

	m1, err := s.GetMatter(id1)
	if err != nil {
		return err
	}

	if relType == "parent" {
		if m1.Parent != id2 {
			return nil // not set, nothing to do
		}
		m1.Parent = ""
		m1.Updated = model.Today()
		return s.WriteMatter(m1)
	}

	m2, err := s.GetMatter(id2)
	if err != nil {
		return err
	}

	var needWriteM1, needWriteM2 bool

	switch relType {
	case "relates":
		if containsString(m1.Relates, id2) {
			m1.Relates = removeString(m1.Relates, id2)
			needWriteM1 = true
		}
		if containsString(m2.Relates, id1) {
			m2.Relates = removeString(m2.Relates, id1)
			needWriteM2 = true
		}
	case "blocks":
		if containsString(m1.Blocks, id2) {
			m1.Blocks = removeString(m1.Blocks, id2)
			needWriteM1 = true
		}
		if containsString(m2.Needs, id1) {
			m2.Needs = removeString(m2.Needs, id1)
			needWriteM2 = true
		}
	case "needs":
		if containsString(m1.Needs, id2) {
			m1.Needs = removeString(m1.Needs, id2)
			needWriteM1 = true
		}
		if containsString(m2.Blocks, id1) {
			m2.Blocks = removeString(m2.Blocks, id1)
			needWriteM2 = true
		}
	}

	if !needWriteM1 && !needWriteM2 {
		return nil
	}

	// Backup m1 for rollback.
	_, backupData, err := s.backupMatterFile(id1)
	if err != nil {
		return err
	}

	if needWriteM1 {
		m1.Updated = model.Today()
		if err := s.WriteMatter(m1); err != nil {
			return err
		}
	}

	if needWriteM2 {
		m2.Updated = model.Today()
		if err := s.WriteMatter(m2); err != nil {
			rollbackPath := filepath.Join(s.mattersDir, m1.Filename)
			os.WriteFile(rollbackPath, backupData, 0644)
			return fmt.Errorf("unlinking failed, rolled back %s: %w", id1, err)
		}
	}

	return nil
}

// RemoveAllReferences removes all references to a matter ID from other matters.
// Used when deleting a matter to clean up dangling references.
func (s *Store) RemoveAllReferences(id string) error {
	all, err := s.ListMatters(nil)
	if err != nil {
		return err
	}

	for _, m := range all {
		changed := false

		if containsString(m.Relates, id) {
			m.Relates = removeString(m.Relates, id)
			changed = true
		}
		if containsString(m.Blocks, id) {
			m.Blocks = removeString(m.Blocks, id)
			changed = true
		}
		if containsString(m.Needs, id) {
			m.Needs = removeString(m.Needs, id)
			changed = true
		}
		if m.Parent == id {
			m.Parent = ""
			changed = true
		}

		if changed {
			m.Updated = model.Today()
			if err := s.WriteMatter(m); err != nil {
				return fmt.Errorf("cleaning references in %s: %w", m.ID, err)
			}
		}
	}
	return nil
}

// matchesFilters checks if a matter matches all the given filters.
func matchesFilters(m *model.Matter, filters map[string]string) bool {
	for k, v := range filters {
		switch k {
		case "status":
			if m.Status != v {
				return false
			}
		case "tag":
			found := false
			for _, t := range m.Tags {
				if t == v {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case "effort":
			if m.Effort != v {
				return false
			}
		case "epic":
			if m.Epic != v {
				return false
			}
		}
	}
	return true
}
