package collect

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"edward/internal/store"
)

type FilesConfig struct {
	Home    string
	Tmp     string
	On      bool
	ReadDir func(string) ([]os.DirEntry, error)
}

func (c FilesConfig) readDir(path string) ([]os.DirEntry, error) {
	if c.ReadDir != nil {
		return c.ReadDir(path)
	}
	return os.ReadDir(path)
}

func ProbeFilesGrant(cfg FilesConfig) error {
	if _, err := cfg.readDir(filepath.Join(cfg.Home, "Downloads")); err != nil {
		return err
	}
	if _, err := cfg.readDir(filepath.Join(cfg.Home, "Desktop")); err != nil {
		return err
	}
	return nil
}

func CollectFiles(cfg FilesConfig) []store.Item {
	if !cfg.On {
		return []store.Item{}
	}
	tmp := cfg.Tmp
	if tmp == "" {
		tmp = "/tmp"
	}
	var items []store.Item
	items = append(items, listTop(cfg, filepath.Join(cfg.Home, "Downloads"))...)
	items = append(items, listTop(cfg, filepath.Join(cfg.Home, "Desktop"))...)
	items = append(items, listTop(cfg, tmp)...)
	return items
}

func listTop(cfg FilesConfig, dir string) []store.Item {
	entries, err := cfg.readDir(dir)
	if err != nil {
		return nil
	}
	now := time.Now()
	var items []store.Item
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		full := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			info, err = os.Lstat(full)
			if err != nil {
				continue
			}
		}
		display := tildePath(cfg.Home, full)
		kind := fileKind(name, info)
		saved := ageAgo(now.Sub(info.ModTime()))
		items = append(items, store.Item{
			ID:     "f-" + display,
			Name:   name,
			Detail: display,
			Meta:   saved,
			Facts: [][]string{
				{"Path", display},
				{"Kind", kind},
				{"Saved", saved},
			},
		})
	}
	return items
}

func tildePath(home, p string) string {
	if home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+"/") {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func fileKind(name string, info fs.FileInfo) string {
	mode := info.Mode()
	ext := strings.ToLower(filepath.Ext(name))
	base := ""
	switch {
	case mode.IsDir() && ext == ".app":
		base = "App bundle"
	case mode.IsDir():
		base = "Folder"
	case ext == ".command" || ext == ".sh":
		base = "Shell script"
	case ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".heic" || ext == ".webp":
		base = "Image"
	case ext == ".pdf":
		base = "PDF"
	case ext == ".dmg":
		base = "Disk image"
	case ext == ".zip" || ext == ".tar" || ext == ".gz" || ext == ".tgz":
		base = "Archive"
	case ext == ".pkg":
		base = "Installer"
	case ext == ".md" || ext == ".txt":
		base = "Text"
	case ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".toml":
		base = "Config"
	case ext == ".plist":
		base = "Startup item"
	case mode&0o111 != 0:
		base = "Program"
	default:
		base = "File"
	}
	if mode.IsDir() && ext != ".app" {
		return base
	}
	return base + ", " + sizeStr(info.Size())
}

func sizeStr(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%d MB", n/(1024*1024))
}

func ageAgo(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d.Seconds())
	switch {
	case s < 60:
		return fmt.Sprintf("%d s ago", s)
	case s < 3600:
		return fmt.Sprintf("%d m ago", s/60)
	case s < 86400:
		return fmt.Sprintf("%d h ago", s/3600)
	case s < 86400*14:
		return fmt.Sprintf("%d d ago", s/86400)
	default:
		return fmt.Sprintf("%d w ago", s/86400/7)
	}
}
