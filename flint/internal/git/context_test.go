package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo := filepath.Join(dir, "test-project")
	os.MkdirAll(repo, 0o755)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init")
	run("checkout", "-b", "main")

	// Create a file and commit
	os.WriteFile(filepath.Join(repo, "hello.go"), []byte("package main"), 0o644)
	run("add", "hello.go")
	run("commit", "-m", "initial commit")

	// Create a dirty file
	os.WriteFile(filepath.Join(repo, "world.go"), []byte("package main"), 0o644)

	return repo
}

func TestContext_returnsBranchAndStatus(t *testing.T) {
	repo := initTestRepo(t)

	ctx, err := Context(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Branch != "main" {
		t.Errorf("branch: got %q, want %q", ctx.Branch, "main")
	}

	if len(ctx.DirtyFiles) == 0 {
		t.Error("expected dirty files, got none")
	}

	found := false
	for _, f := range ctx.DirtyFiles {
		if strings.Contains(f, "world.go") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected world.go in dirty files, got %v", ctx.DirtyFiles)
	}

	if len(ctx.RecentCommits) == 0 {
		t.Error("expected recent commits, got none")
	}

	if !strings.Contains(ctx.RecentCommits[0], "initial commit") {
		t.Errorf("expected 'initial commit' in recent commits, got %q", ctx.RecentCommits[0])
	}
}
