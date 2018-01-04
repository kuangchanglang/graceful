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
	"sync"
	"syscall"
	"time"

	"code.byted.org/gopkg/pkg/log"
)

var (
	ErrNoServers = errors.New("no servers")
)

type worker struct {
	handlers []http.Handler
	servers  []server
	opt      *option
	sync.Mutex
}

type server struct {
	http.Server
	listener net.Listener
}

func (w *worker) run() error {
	// init servers with fds from master
	err := w.initServers()
	if err != nil {
		return err
	}

	// start http servers
	err = w.startServers()
	if err != nil {
		return err
	}

	go w.watchMaster()

	// waitSignal
	w.waitSignal()
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

func (w *worker) startServers() error {
	if len(w.servers) == 0 {
		return ErrNoServers
	}
	for i := 0; i < len(w.servers); i++ {
		s := w.servers[i]
		go func() {
			if err := s.Serve(s.listener); err != nil {
				log.Errorf("http Serve error: %v", err)
			}
		}()
	}

	return nil
}

// watchMaster to monitor if master dead
func (w *worker) watchMaster() error {
	for {
		// if parent id change to 1, it means parent is dead
		if os.Getppid() == 1 {
			log.Infof("master dead, stop worker")
			w.stop()
			break
		}
		time.Sleep(w.opt.watchInterval)
	}
	return nil
}

func (w *worker) waitSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGKILL)
	<-ch
	w.stop()
}

// TODO: shutdown in parallel
func (w *worker) stop() {
	w.Lock()
	defer w.Unlock()
	for _, server := range w.servers {
		ctx, cancel := context.WithTimeout(context.TODO(), w.opt.stopTimeout)
		defer cancel()
		err := server.Shutdown(ctx)
		if err != nil {
			log.Errorf("shutdown server error: %v", err)
		}
	}
	os.Exit(0)
}
