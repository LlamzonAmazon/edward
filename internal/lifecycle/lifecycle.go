package lifecycle

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const label = "com.edward.agent"

// StopAgent removes the user LaunchAgent so EDwaRd does not start at login.
// The process is left running; the caller should exit after responding.
func StopAgent(dataDir string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("deactivate is macOS only in this build")
	}
	plist := plistPath()
	uid := uidString()
	domain := "gui/" + uid
	if fileExists(plist) {
		_ = exec.Command("/bin/launchctl", "bootout", domain, plist).Run()
	}
	_ = exec.Command("/bin/launchctl", "bootout", domain+"/"+label).Run()
	if fileExists(plist) {
		if err := os.Remove(plist); err != nil {
			return fmt.Errorf("remove LaunchAgent: %w", err)
		}
	}
	_ = dataDir
	return nil
}

// Uninstall stops the LaunchAgent, removes agent skill files and the PATH link,
// and schedules deletion of the data directory after this process exits.
func Uninstall(dataDir string) error {
	if err := StopAgent(dataDir); err != nil {
		return err
	}
	home := homeDir()
	stripSentinels(filepath.Join(home, ".claude", "CLAUDE.md"))
	stripSentinels(filepath.Join(home, ".codex", "AGENTS.md"))
	for _, d := range []string{
		filepath.Join(home, ".cursor", "skills", "edward"),
		filepath.Join(home, ".claude", "skills", "edward"),
		filepath.Join(home, ".codex", "skills", "edward"),
		filepath.Join(home, ".openclaw", "skills", "edward"),
	} {
		_ = os.RemoveAll(d)
	}
	link := filepath.Join(home, ".local", "bin", "edward")
	if target, err := os.Readlink(link); err == nil {
		bin := filepath.Join(dataDir, "EDwaRd.app", "Contents", "MacOS", "edward")
		if target == bin || strings.HasSuffix(target, "/EDwaRd.app/Contents/MacOS/edward") {
			_ = os.Remove(link)
		}
	}
	scheduleRemove(dataDir)
	return nil
}

func ExitSoon() {
	time.AfterFunc(250*time.Millisecond, func() { os.Exit(0) })
}

func plistPath() string {
	if v := os.Getenv("EDWARD_LAUNCH_AGENTS"); v != "" {
		return filepath.Join(v, label+".plist")
	}
	return filepath.Join(homeDir(), "Library", "LaunchAgents", label+".plist")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.HomeDir
}

func uidString() string {
	u, err := user.Current()
	if err != nil {
		return strconv.Itoa(os.Getuid())
	}
	return u.Uid
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func stripSentinels(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	s := string(b)
	start := strings.Index(s, "<!-- edward:start -->")
	end := strings.Index(s, "<!-- edward:end -->")
	if start < 0 || end < start {
		return
	}
	end += len("<!-- edward:end -->")
	out := strings.TrimRight(s[:start], " \t\r\n") + s[end:]
	out = strings.TrimLeft(out, "\n")
	if strings.TrimSpace(out) == "" {
		_ = os.Remove(path)
		return
	}
	_ = os.WriteFile(path, []byte(out), 0o644)
}

func scheduleRemove(dataDir string) {
	if dataDir == "" || dataDir == "/" || dataDir == homeDir() {
		return
	}
	script := fmt.Sprintf("sleep 1; rm -rf %s", shellQuote(dataDir))
	cmd := exec.Command("/bin/sh", "-c", script)
	_ = cmd.Start()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
