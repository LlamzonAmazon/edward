package collect

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

func commandEnv() []string {
	out := []string{"LANG=C", "LC_ALL=C"}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "LANG=") || strings.HasPrefix(e, "LC_ALL=") {
			continue
		}
		out = append(out, e)
	}
	return out
}

func runCmd(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = commandEnv()
	return cmd.Output()
}

func runCmdCombined(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = commandEnv()
	return cmd.CombinedOutput()
}
