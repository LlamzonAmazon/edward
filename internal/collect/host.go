package collect

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

func CurrentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if v := os.Getenv("USER"); v != "" {
		return v
	}
	return ""
}

func CurrentHome() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		return u.HomeDir
	}
	return ""
}

func DataDir() string {
	if v := os.Getenv("EDWARD_HOME"); v != "" {
		return v
	}
	return filepath.Join(CurrentHome(), "Library", "Application Support", "EDwaRd")
}

func CollectHost() (name, osName, userName string) {
	userName = CurrentUser()
	name = hostname()
	if out, err := runCmd(2*time.Second, "/usr/sbin/scutil", "--get", "ComputerName"); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			name = s
		}
	}
	osName = "macOS"
	if out, err := runCmd(2*time.Second, "/usr/bin/sw_vers"); err == nil {
		ver := ""
		prod := ""
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "ProductName:") {
				prod = strings.TrimSpace(strings.TrimPrefix(line, "ProductName:"))
			}
			if strings.HasPrefix(line, "ProductVersion:") {
				ver = strings.TrimSpace(strings.TrimPrefix(line, "ProductVersion:"))
			}
		}
		if prod == "" {
			prod = "macOS"
		}
		if ver != "" {
			osName = prod + " " + ver
		} else {
			osName = prod
		}
	}
	return name, osName, userName
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}
