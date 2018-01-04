package main

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/kuangchanglang/graceful"
)

type handler struct {
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, port: %v, %q", r.Host, html.EscapeString(r.URL.Path))
}

func main() {
	graceful.ListenAndServe(":9222", &handler{})
}

func listenMultiAddrs() {
	server := graceful.NewServer()
	server.Register("0.0.0.0:9223", &handler{})
	server.Register("0.0.0.0:9224", &handler{})
	server.Register("0.0.0.0:9225", &handler{})
	err := server.Run()
	fmt.Printf("error: %v\n", err)
}

func setReloadSignal() {
	server := graceful.NewServer(
		graceful.WithReloadSignals([]os.Signal{syscall.SIGUSR2}),
		graceful.WithStopSignals([]os.Signal{syscall.SIGINT}),
		graceful.WithStopTimeout(time.Minute),
		graceful.WithWatchInterval(10*time.Second),
	)
	server.Register("0.0.0.0:9226", &handler{})
	server.Run()
}
