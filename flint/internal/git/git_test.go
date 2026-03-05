package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoName_returnsTopLevelDirName(t *testing.T) {
	// Create a fake git repo in a temp dir
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "my-project")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(repoDir, "src", "pkg")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// From the repo root
	name, err := RepoName(repoDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "my-project" {
		t.Errorf("got %q, want %q", name, "my-project")
	}

	// From a subdirectory
	name, err = RepoName(subDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "my-project" {
		t.Errorf("got %q, want %q", name, "my-project")
	}
}

func TestRepoName_errorsOutsideRepo(t *testing.T) {
	tmp := t.TempDir()
	_, err := RepoName(tmp)
	if err == nil {
		t.Error("expected error outside git repo, got nil")
	}
}
