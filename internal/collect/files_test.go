package collect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesOffDoesNotListDownloadsOrDesktop(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Downloads"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "Desktop"), 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(home, "Downloads", "secret.txt")
	if err := os.WriteFile(secret, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	desktop := filepath.Join(home, "Desktop", "note.md")
	if err := os.WriteFile(desktop, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	var touched []string
	cfg := FilesConfig{
		Home: home,
		Tmp:  t.TempDir(),
		On:   false,
		ReadDir: func(path string) ([]os.DirEntry, error) {
			touched = append(touched, path)
			return os.ReadDir(path)
		},
	}
	items := CollectFiles(cfg)
	if len(items) != 0 {
		t.Fatalf("expected no items with filesOn false, got %#v", items)
	}
	for _, p := range touched {
		if strings.Contains(p, "Downloads") || strings.Contains(p, "Desktop") {
			t.Fatalf("listed %s while filesOn is false", p)
		}
	}
	for _, it := range items {
		if strings.Contains(it.ID, "secret") || strings.Contains(it.Detail, "Downloads") || strings.Contains(it.Detail, "Desktop") {
			t.Fatalf("Downloads/Desktop leaked: %#v", it)
		}
	}
}

func TestFilesGrantDeniedMissingDir(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Downloads"), 0o700); err != nil {
		t.Fatal(err)
	}
	err := ProbeFilesGrant(FilesConfig{Home: home})
	if err == nil {
		t.Fatal("expected grant probe to fail with missing Desktop")
	}

	data := t.TempDir()
	r := NewRunner(data)
	r.home = home
	st := r.SetFiles(true)
	if st.FilesOn {
		t.Fatal("filesOn should be false when listing fails")
	}
	if st.FilesGrant != "denied" {
		t.Fatalf("filesGrant %q, want denied", st.FilesGrant)
	}
}
