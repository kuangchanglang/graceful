package graceful

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	workerStopSignal = syscall.SIGTERM
)

var (
	ErrNoServers = errors.New("no servers")
)

type worker struct {
	handlers []http.Handler
	servers  []server
	opt      *option
	stopCh   chan struct{}
	sync.Mutex
}

type server struct {
	*http.Server
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

	oldWorkerPid, err := strconv.Atoi(os.Getenv(EnvOldWorkerPid))
	if err == nil && oldWorkerPid > 1 {
		// tell old worker i'm ready, you should go away
		err = syscall.Kill(oldWorkerPid, workerStopSignal)
		if err != nil {
			// unexpected: kill old worker fail
			log.Printf("[warning] kill old worker error: %v\n", err)
		}
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

	if len(w.handlers) != numFDs {
		return fmt.Errorf("handler number does not match numFDs, %v!=%v", len(w.handlers), numFDs)
	}

	for i := 0; i < numFDs; i++ {
		f := os.NewFile(uintptr(3+i), "") // fd start from 3
		l, err := net.FileListener(f)
		if err != nil {
			return fmt.Errorf("failed to inherit file descriptor: %d", i)
		}
		server := server{
			Server: &http.Server{
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
				log.Printf("http Serve error: %v\n", err)
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
			log.Printf("master dead, stop worker\n")
			w.stop()
			break
		}
		time.Sleep(w.opt.watchInterval)
	}
	w.stopCh <- struct{}{}
	return nil
}

func (w *worker) waitSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, workerStopSignal)
	select {
	case sig := <-ch:
		log.Printf("worker got signal: %v\n", sig)
	case <-w.stopCh:
		log.Printf("stop worker")
	}

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
			log.Printf("shutdown server error: %v\n", err)
		}
	}
}
