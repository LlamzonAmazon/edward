package lifecycle

import (
	"errors"
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

	"github.com/LlamzonAmazon/edward/internal/agents"
)

const Label = "com.edward.agent"

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

// Uninstall is the page's Delete button. It removes skill files, blocks and the
// PATH link while this process is alive, then starts `edward uninstall` in a
// detached session. That command boots out this LaunchAgent and deletes the
// data directory after this process has answered and exited.
func Uninstall(dataDir string) error {
	home := homeDir()
	agents.Cleanup(home)
	removePathLink(home, dataDir)
	bin := filepath.Join(dataDir, "EDwaRd.app", "Contents", "MacOS", "edward")
	scheduleDetached(fmt.Sprintf("sleep 1\n%s uninstall >/dev/null 2>&1", shellQuote(bin)))
	return nil
}

// UninstallNow is `edward uninstall`: it removes everything EDwaRd installed and
// returns one line per path. A second run prints "already absent" for each path
// and returns no error.
func UninstallNow(dataDir string) ([]string, error) {
	home := homeDir()
	bin := filepath.Join(dataDir, "EDwaRd.app", "Contents", "MacOS", "edward")
	var lines []string
	var failed bool
	add := func(status, what string) {
		lines = append(lines, status+" "+what)
		if status == agents.StatusLeft {
			failed = true
		}
	}

	add(bootout(), "LaunchAgent "+Label)
	add(stopProcesses(bin), "process "+bin)
	add(removeFile(plistPath()), tilde(home, plistPath()))
	for _, l := range agents.Cleanup(home) {
		lines = append(lines, l)
		if strings.HasPrefix(l, agents.StatusLeft) && !strings.Contains(l, "not EDwaRd's") {
			failed = true
		}
	}
	if status, why := removePathLink(home, dataDir); why != "" {
		lines = append(lines, status+" ~/.local/bin/edward ("+why+")")
	} else {
		add(status, "~/.local/bin/edward")
	}
	add(removeTree(home, LogDir()), tilde(home, LogDir()))
	add(removeTree(home, dataDir), tilde(home, dataDir))

	lines = append(lines,
		"not touched: the rest of ~/.claude/CLAUDE.md and ~/.codex/AGENTS.md, other skills, project files, other LaunchAgents",
		"not touched: macOS privacy grants. Remove an EDwaRd row in System Settings > Privacy & Security if one is there.",
	)
	if failed {
		return lines, errors.New("some EDwaRd files were left in place")
	}
	return lines, nil
}

// LogDir is where the LaunchAgent writes stdout and stderr.
func LogDir() string {
	if v := os.Getenv("EDWARD_LOGS"); v != "" {
		return v
	}
	return filepath.Join(homeDir(), "Library", "Logs", "EDwaRd")
}

func ExitSoon() {
	time.AfterFunc(500*time.Millisecond, func() { os.Exit(0) })
}

func bootout() string {
	if runtime.GOOS != "darwin" {
		return agents.StatusAbsent
	}
	domain := "gui/" + uidString()
	loaded := false
	if _, err := os.Stat(plistPath()); err == nil {
		if exec.Command("launchctl", "bootout", domain, plistPath()).Run() == nil {
			loaded = true
		}
	}
	// With EDWARD_LAUNCH_AGENTS set (tests), never touch the real user label.
	if os.Getenv("EDWARD_LAUNCH_AGENTS") == "" {
		if exec.Command("launchctl", "print", domain+"/"+Label).Run() == nil {
			_ = exec.Command("launchctl", "bootout", domain+"/"+Label).Run()
			loaded = true
		}
	}
	if loaded {
		return agents.StatusRemoved
	}
	return agents.StatusAbsent
}

// stopProcesses stops processes running this exact binary, other than this one.
func stopProcesses(bin string) string {
	pids := pidsFor(bin)
	if len(pids) == 0 {
		return agents.StatusAbsent
	}
	for _, p := range pids {
		_ = syscall.Kill(p, syscall.SIGTERM)
	}
	for i := 0; i < 20 && len(pidsFor(bin)) > 0; i++ {
		time.Sleep(100 * time.Millisecond)
	}
	for _, p := range pidsFor(bin) {
		_ = syscall.Kill(p, syscall.SIGKILL)
	}
	if len(pidsFor(bin)) > 0 {
		return agents.StatusLeft
	}
	return agents.StatusRemoved
}

func pidsFor(bin string) []int {
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var pids []int
	for _, l := range strings.Split(string(out), "\n") {
		l = strings.TrimSpace(l)
		sp := strings.IndexByte(l, ' ')
		if sp < 0 {
			continue
		}
		pid, err := strconv.Atoi(l[:sp])
		if err != nil || pid == self {
			continue
		}
		cmd := strings.TrimSpace(l[sp:])
		if cmd == bin || strings.HasPrefix(cmd, bin+" ") {
			pids = append(pids, pid)
		}
	}
	return pids
}

func removeFile(path string) string {
	err := os.Remove(path)
	switch {
	case err == nil:
		return agents.StatusRemoved
	case os.IsNotExist(err):
		return agents.StatusAbsent
	default:
		return agents.StatusLeft
	}
}

// removeTree deletes a directory EDwaRd created. It refuses the home folder
// and the root.
func removeTree(home, dir string) string {
	clean := filepath.Clean(dir)
	if home == "" || dir == "" || clean == "/" || clean == filepath.Clean(home) {
		return agents.StatusLeft
	}
	if _, err := os.Lstat(clean); os.IsNotExist(err) {
		return agents.StatusAbsent
	}
	if err := os.RemoveAll(clean); err != nil {
		return agents.StatusLeft
	}
	return agents.StatusRemoved
}

// removePathLink removes ~/.local/bin/edward only when it links to an EDwaRd binary.
func removePathLink(home, dataDir string) (status, why string) {
	link := filepath.Join(home, ".local", "bin", "edward")
	target, err := os.Readlink(link)
	if err != nil {
		if _, statErr := os.Lstat(link); os.IsNotExist(statErr) {
			return agents.StatusAbsent, ""
		}
		return agents.StatusLeft, "not EDwaRd's"
	}
	bin := filepath.Join(dataDir, "EDwaRd.app", "Contents", "MacOS", "edward")
	if target == bin || strings.HasSuffix(target, "/EDwaRd.app/Contents/MacOS/edward") {
		if os.Remove(link) == nil {
			return agents.StatusRemoved, ""
		}
		return agents.StatusLeft, ""
	}
	return agents.StatusLeft, "not EDwaRd's"
}

func tilde(home, p string) string {
	if home != "" && strings.HasPrefix(p, home+string(os.PathSeparator)) {
		return "~" + p[len(home):]
	}
	return p
}

func stopScript() string {
	plist := plistPath()
	domain := "gui/" + uidString()
	lines := []string{
		"sleep 1",
		fmt.Sprintf("launchctl bootout %s %s 2>/dev/null || true", shellQuote(domain), shellQuote(plist)),
	}
	// When EDWARD_LAUNCH_AGENTS is set (tests), only touch that plist. Never bootout the real user label.
	if os.Getenv("EDWARD_LAUNCH_AGENTS") == "" {
		lines = append(lines, fmt.Sprintf("launchctl bootout %s/%s 2>/dev/null || true", shellQuote(domain), shellQuote(Label)))
	}
	lines = append(lines, fmt.Sprintf("rm -f %s", shellQuote(plist)))
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
	_ = cmd.Start()
}

func plistPath() string {
	if v := os.Getenv("EDWARD_LAUNCH_AGENTS"); v != "" {
		return filepath.Join(v, Label+".plist")
	}
	return filepath.Join(homeDir(), "Library", "LaunchAgents", Label+".plist")
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

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
