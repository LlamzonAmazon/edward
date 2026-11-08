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
	"syscall"
	"time"
)

const label = "com.edward.agent"

// StopAgent schedules removal of the user LaunchAgent after this process exits.
// It must not launchctl bootout in-process: bootout kills this job before the
// HTTP response and before ExitSoon can run.
func StopAgent(dataDir string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("deactivate is macOS only in this build")
	}
	_ = dataDir
	scheduleDetached(stopScript())
	return nil
}

// Uninstall removes skill files, sentinel blocks, and the PATH link while this
// process is still alive, then schedules LaunchAgent teardown and deletion of
// the data directory after exit. Bootout runs only in that detached script so
// cleanup is not cut short when launchctl stops the job.
func Uninstall(dataDir string) error {
	home := homeDir()
	stripSentinels(filepath.Join(home, ".claude", "CLAUDE.md"))
	stripSentinels(filepath.Join(home, ".codex", "AGENTS.md"))
	for _, d := range skillDirs(home) {
		_ = os.RemoveAll(d)
	}
	removePathLink(home, dataDir)
	scheduleDetached(uninstallScript(dataDir))
	return nil
}

func ExitSoon() {
	time.AfterFunc(500*time.Millisecond, func() { os.Exit(0) })
}

func skillDirs(home string) []string {
	return []string{
		filepath.Join(home, ".cursor", "skills", "edward"),
		filepath.Join(home, ".claude", "skills", "edward"),
		filepath.Join(home, ".codex", "skills", "edward"),
		filepath.Join(home, ".openclaw", "skills", "edward"),
	}
}

func removePathLink(home, dataDir string) {
	link := filepath.Join(home, ".local", "bin", "edward")
	target, err := os.Readlink(link)
	if err != nil {
		return
	}
	bin := filepath.Join(dataDir, "EDwaRd.app", "Contents", "MacOS", "edward")
	if target == bin || strings.Contains(target, "/EDwaRd.app/Contents/MacOS/edward") || strings.HasPrefix(target, dataDir+string(os.PathSeparator)) {
		_ = os.Remove(link)
	}
}

func stopScript() string {
	plist := plistPath()
	uid := uidString()
	domain := "gui/" + uid
	lines := []string{
		"sleep 1",
		fmt.Sprintf("launchctl bootout %s %s 2>/dev/null || true", shellQuote(domain), shellQuote(plist)),
	}
	// When EDWARD_LAUNCH_AGENTS is set (tests), only touch that plist. Never bootout the real user label.
	if os.Getenv("EDWARD_LAUNCH_AGENTS") == "" {
		lines = append(lines, fmt.Sprintf("launchctl bootout %s/%s 2>/dev/null || true", shellQuote(domain), shellQuote(label)))
	}
	lines = append(lines, fmt.Sprintf("rm -f %s", shellQuote(plist)))
	return strings.Join(lines, "\n")
}

func uninstallScript(dataDir string) string {
	if dataDir == "" || dataDir == "/" || dataDir == homeDir() {
		return stopScript()
	}
	home := homeDir()
	plist := plistPath()
	uid := uidString()
	domain := "gui/" + uid
	bin := filepath.Join(dataDir, "EDwaRd.app", "Contents", "MacOS", "edward")
	link := filepath.Join(home, ".local", "bin", "edward")
	lines := []string{
		"sleep 2",
		fmt.Sprintf("launchctl bootout %s %s 2>/dev/null || true", shellQuote(domain), shellQuote(plist)),
	}
	if os.Getenv("EDWARD_LAUNCH_AGENTS") == "" {
		lines = append(lines, fmt.Sprintf("launchctl bootout %s/%s 2>/dev/null || true", shellQuote(domain), shellQuote(label)))
	}
	lines = append(lines,
		fmt.Sprintf("rm -f %s", shellQuote(plist)),
		fmt.Sprintf("pkill -f %s 2>/dev/null || true", shellQuote(bin)),
		"sleep 1",
		fmt.Sprintf("rm -rf %s", shellQuote(dataDir)),
		fmt.Sprintf("rm -f %s", shellQuote(link)),
	)
	for _, d := range skillDirs(home) {
		lines = append(lines, fmt.Sprintf("rm -rf %s", shellQuote(d)))
	}
	return strings.Join(lines, "\n")
}

func scheduleDetached(script string) {
	if script == "" {
		return
	}
	// Tests set this so Uninstall can assert file cleanup without spawning a live shell.
	if os.Getenv("EDWARD_SKIP_DETACH") != "" {
		return
	}
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	_ = cmd.Start()
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

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
