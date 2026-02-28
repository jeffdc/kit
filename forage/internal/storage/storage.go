package storage

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"forage/internal/model"

	"gopkg.in/yaml.v3"
)

type Store struct {
	root     string
	booksDir string
}

func New(root string) (*Store, error) {
	booksDir := filepath.Join(root, "books")
	if err := os.MkdirAll(booksDir, 0755); err != nil {
		return nil, err
	}
	return &Store{root: root, booksDir: booksDir}, nil
}

func DefaultRoot() string {
	if dir := os.Getenv("FORAGE_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forage")
}

func (s *Store) Root() string     { return s.root }
func (s *Store) BooksDir() string { return s.booksDir }

// Slugify converts a title to a URL-friendly slug.
func Slugify(title string) string {
	str := strings.ToLower(title)
	str = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, str)
	re := regexp.MustCompile(`-+`)
	str = re.ReplaceAllString(str, "-")
	str = strings.Trim(str, "-")
	return str
}

func (s *Store) generateID(title, author string) (string, error) {
	t := time.Now()
	for i := range 100 {
		ts := t.Add(time.Duration(i) * time.Nanosecond).Format(time.RFC3339Nano)
		h := sha256.Sum256([]byte(title + author + ts))
		id := fmt.Sprintf("%x", h[:2])
		if _, err := s.findBookFile(id); err != nil {
			return id, nil
		}
	}
	return "", fmt.Errorf("could not generate unique ID after 100 attempts")
}

func (s *Store) CreateBook(title, author string, meta map[string]string) (*model.Book, error) {
	id, err := s.generateID(title, author)
	if err != nil {
		return nil, err
	}

	today := time.Now().Format("2006-01-02")
	b := &model.Book{
		ID:        id,
		Title:     title,
		Author:    author,
		Status:    "wishlist",
		DateAdded: today,
	}

	if meta != nil {
		if err := applyMeta(b, meta); err != nil {
			return nil, err
		}
	}

	b.Filename = fmt.Sprintf("%s-%s.md", id, Slugify(title))

	if err := s.writeBook(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) GetBook(id string) (*model.Book, error) {
	path, err := s.findBookFile(id)
	if err != nil {
		return nil, err
	}
	return s.readBookFile(path)
}

func (s *Store) ListBooks(filters map[string]string) ([]model.Book, error) {
	entries, err := os.ReadDir(s.booksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var books []model.Book
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := s.readBookFile(filepath.Join(s.booksDir, e.Name()))
		if err != nil {
			continue
		}
		if matchesFilters(b, filters) {
			books = append(books, *b)
		}
	}
	return books, nil
}

func (s *Store) SearchBooks(query string) ([]model.Book, error) {
	all, err := s.ListBooks(nil)
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	var results []model.Book
	for _, b := range all {
		if strings.Contains(strings.ToLower(b.Title), q) ||
			strings.Contains(strings.ToLower(b.Author), q) ||
			strings.Contains(strings.ToLower(b.Body), q) ||
			strings.Contains(strings.ToLower(strings.Join(b.Tags, " ")), q) {
			results = append(results, b)
		}
	}
	return results, nil
}

// validKeys are the fields that can be set via UpdateBook.
var validKeys = map[string]bool{
	"title": true, "author": true, "status": true,
	"rating": true, "tags": true, "date_read": true,
}

func (s *Store) UpdateBook(id, key, value string) (*model.Book, error) {
	if !validKeys[key] {
		return nil, fmt.Errorf("invalid key: %s (valid: title, author, status, rating, tags, date_read)", key)
	}

	b, err := s.GetBook(id)
	if err != nil {
		return nil, err
	}

	oldFilename := b.Filename

	if err := applyMeta(b, map[string]string{key: value}); err != nil {
		return nil, err
	}

	if key == "title" {
		newFilename := fmt.Sprintf("%s-%s.md", b.ID, Slugify(b.Title))
		if newFilename != oldFilename {
			oldPath := filepath.Join(s.booksDir, oldFilename)
			b.Filename = newFilename
			if err := s.writeBook(b); err != nil {
				return nil, err
			}
			os.Remove(oldPath)
			return b, nil
		}
	}

	if err := s.writeBook(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) DeleteBook(id string) error {
	path, err := s.findBookFile(id)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s *Store) findBookFile(id string) (string, error) {
	entries, err := os.ReadDir(s.booksDir)
	if err != nil {
		return "", fmt.Errorf("book not found: %s", id)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), id+"-") && strings.HasSuffix(e.Name(), ".md") {
			return filepath.Join(s.booksDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("book not found: %s", id)
}

func (s *Store) readBookFile(path string) (*model.Book, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(path)
	id := strings.SplitN(filename, "-", 2)[0]

	b := &model.Book{
		ID:       id,
		Filename: filename,
	}

	content := string(data)
	if strings.HasPrefix(content, "---\n") {
		parts := strings.SplitN(content[4:], "\n---\n", 2)
		if len(parts) == 2 {
			if err := yaml.Unmarshal([]byte(parts[0]), b); err != nil {
				return nil, fmt.Errorf("parsing frontmatter of %s: %w", filename, err)
			}
			b.Body = strings.TrimSpace(parts[1])
		}
	}

	// Restore derived fields that yaml:"-" skipped
	b.ID = id
	b.Filename = filename

	return b, nil
}

func (s *Store) writeBook(b *model.Book) error {
	fm := buildFrontmatter(b)

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return err
	}

	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(fmBytes)
	buf.WriteString("---\n")
	if b.Body != "" {
		buf.WriteString("\n" + b.Body + "\n")
	}

	path := filepath.Join(s.booksDir, b.Filename)
	return os.WriteFile(path, []byte(buf.String()), 0644)
}

func buildFrontmatter(b *model.Book) yaml.Node {
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
		seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, v := range values {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
		}
		doc.Content = append(doc.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			seq,
		)
	}

	addField("title", b.Title)
	addField("author", b.Author)
	addField("status", b.Status)
	addStringSlice("tags", b.Tags)
	if b.Rating > 0 {
		doc.Content = append(doc.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "rating"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: strconv.Itoa(b.Rating), Tag: "!!int"},
		)
	}
	addField("date_added", b.DateAdded)
	addField("date_read", b.DateRead)

	return doc
}

func applyMeta(b *model.Book, meta map[string]string) error {
	for k, v := range meta {
		switch k {
		case "title":
			b.Title = v
		case "author":
			b.Author = v
		case "status":
			if !model.ValidStatus(v) {
				return fmt.Errorf("invalid status: %s (valid: wishlist, reading, read, dropped)", v)
			}
			b.Status = v
		case "rating":
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid rating: %s", v)
			}
			b.Rating = n
		case "tags":
			tags := strings.Split(v, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
			b.Tags = tags
		case "date_read":
			b.DateRead = v
		case "body":
			b.Body = v
		}
	}
	return nil
}

func matchesFilters(b *model.Book, filters map[string]string) bool {
	for k, v := range filters {
		switch k {
		case "status":
			if b.Status != v {
				return false
			}
		case "tag":
			if !slices.Contains(b.Tags, v) {
				return false
			}
		case "author":
			if !strings.EqualFold(b.Author, v) {
				return false
			}
		}
	}
	return true
}
