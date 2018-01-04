package graceful

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"code.byted.org/gopkg/pkg/log"
)

var (
	ErrNoServers = errors.New("no servers")
)

type worker struct {
	handlers []http.Handler
	servers  []server
}

type server struct {
	http.Server
	listener net.Listener
}

func (w *worker) run() error {

	// listening fds
	err := w.initServers()
	if err != nil {
		return err
	}

	err = w.start()
	if err != nil {
		return err
	}
	// waitSignal
	w.waitSignal()
	return nil
}

func (w *worker) waitSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGKILL)
	<-ch
	w.stop()
}

func (w *worker) stop() {
	for _, server := range w.servers {
		err := server.Shutdown(context.TODO())
		if err != nil {
			log.Error("shutdown server error: %v", err)
		}
	}
}

func (w *worker) start() error {
	if len(w.servers) == 0 {
		return ErrNoServers
	}
	for i := 1; i < len(w.servers); i++ {
		s := w.servers[i]
		go func() {
			if err := s.Serve(s.listener); err != nil {
				log.Error("http Serve error: %v", err)
			}
		}()
	}

	return nil
}

func (w *worker) initServers() error {
	numFDs, err := strconv.Atoi(os.Getenv(EnvNumFD))
	if err != nil {
		return fmt.Errorf("invalid %s integer", EnvNumFD)
	}

	for i := 0; i < numFDs; i++ {
		f := os.NewFile(uintptr(3+i), "") // fd start from 3
		l, err := net.FileListener(f)
		if err != nil {
			return fmt.Errorf("failed to inherit file descriptor: %d", i)
		}
		server := server{
			Server: http.Server{
				Handler: w.handlers[i],
			},
			listener: l,
		}
		w.servers = append(w.servers, server)
	}
	return nil
}
