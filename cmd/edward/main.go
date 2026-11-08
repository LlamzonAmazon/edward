package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"edward/internal/collect"
	"edward/internal/server"
)

func main() {
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
	ln, err := server.Listen("127.0.0.1:43393")
	if err != nil {
		fmt.Fprintf(os.Stderr, "EDwaRd: cannot bind 127.0.0.1:43393: %v\n", err)
		os.Exit(1)
	}
	go r.Loop()
	srv := &http.Server{Handler: server.New(r)}
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
