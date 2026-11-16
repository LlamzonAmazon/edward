package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/LlamzonAmazon/edward/internal/agents"
	"github.com/LlamzonAmazon/edward/internal/collect"
	"github.com/LlamzonAmazon/edward/internal/lifecycle"
	"github.com/LlamzonAmazon/edward/internal/server"
	"github.com/LlamzonAmazon/edward/internal/version"
)

const (
	addr    = "127.0.0.1:43393"
	pageURL = "http://" + addr
)

const usage = `EDwaRd %s: a local picture of this Mac's attack surface for your agent.

Usage: edward <command>

  status     Show whether EDwaRd is running, and where its files are
  open       Open the page (` + pageURL + `)
  clear      Clear the findings whose values are given on stdin, one per line
             (--undo raises them again)
  uninstall  Remove EDwaRd from this Mac: program, login item, store, logs, skills
  version    Print the version
  serve      Run the collector and page (the login item runs this)
`

func main() {
	cmd := ""
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	// Login items written by earlier builds start the binary with no arguments.
	if cmd == "" && os.Getenv("XPC_SERVICE_NAME") == lifecycle.Label {
		cmd = "serve"
	}
	switch cmd {
	case "serve":
		serve()
	case "status":
		status()
	case "open":
		if err := exec.Command("/usr/bin/open", pageURL).Run(); err != nil {
			fmt.Fprintln(os.Stderr, "EDwaRd: could not open "+pageURL)
			os.Exit(1)
		}
	case "clear":
		os.Exit(clearFindings(os.Args[2:]))
	case "uninstall":
		lines, err := lifecycle.UninstallNow(collect.DataDir())
		for _, l := range lines {
			fmt.Println(l)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "EDwaRd: "+err.Error())
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Println(version.String)
	case "", "help", "--help", "-h":
		fmt.Printf(usage, version.String)
	default:
		fmt.Fprintf(os.Stderr, "edward: unknown command %q\n\n", cmd)
		fmt.Fprintf(os.Stderr, usage, version.String)
		os.Exit(2)
	}
}

func serve() {
	dataDir := collect.DataDir()
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "EDwaRd: data dir: %v\n", err)
		os.Exit(1)
	}
	r := collect.NewRunner(dataDir)
	if err := r.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "EDwaRd: load store: %v\n", err)
		os.Exit(1)
	}
	ln, err := server.Listen(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "EDwaRd: cannot bind %s: %v\n", addr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "EDwaRd %s: serving %s, store in %s\n", version.String, pageURL, dataDir)
	go r.Loop()
	srv := &http.Server{
		Handler:           server.New(r),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "EDwaRd: server: %v\n", err)
			os.Exit(1)
		}
	}()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	_ = srv.Close()
}

func status() {
	home, _ := os.UserHomeDir()
	running := false
	c := http.Client{Timeout: time.Second}
	if res, err := c.Get(pageURL + "/"); err == nil {
		res.Body.Close()
		running = res.StatusCode == http.StatusOK
	}
	fmt.Printf("EDwaRd %s\n", version.String)
	if running {
		fmt.Printf("running   %s\n", pageURL)
	} else {
		fmt.Printf("stopped   nothing answers on %s\n", pageURL)
	}
	fmt.Printf("store     %s\n", filepath.Join(collect.DataDir(), "store.json"))
	fmt.Printf("logs      %s\n", filepath.Join(lifecycle.LogDir(), "edward.log"))
	for _, dir := range agents.AllSkillDirs() {
		b, err := os.ReadFile(filepath.Join(home, dir, "SKILL.md"))
		if err == nil && agents.Owned(b) {
			fmt.Printf("skill     ~/%s/SKILL.md\n", dir)
		}
	}
}
