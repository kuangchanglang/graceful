// +build !windows

package graceful

import (
	"net/http"
	"os"
	"syscall"
	"time"
)

// constants
const (
	EnvWorker       = "GRACEFUL_WORKER"
	EnvNumFD        = "GRACEFUL_NUMFD"
	EnvOldWorkerPid = "GRACEFUL_OLD_WORKER_PID"
	EnvParentPid    = "GRACEFUL_PARENT_PID"
	ValWorker       = "1"
)

var (
	defaultWatchInterval = time.Second
	defaultStopTimeout   = 20 * time.Second
	defaultReloadSignals = []syscall.Signal{syscall.SIGHUP, syscall.SIGUSR1}
	defaultStopSignals   = []syscall.Signal{syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT}

	StartedAt time.Time
)

type option struct {
	reloadSignals []syscall.Signal
	stopSignals   []syscall.Signal
	watchInterval time.Duration
	stopTimeout   time.Duration
}

type Option func(o *option)

// WithReloadSignals set reload signals, otherwise, default ones are used
func WithReloadSignals(sigs []syscall.Signal) Option {
	return func(o *option) {
		o.reloadSignals = sigs
	}
}

// WithStopSignals set stop signals, otherwise, default ones are used
func WithStopSignals(sigs []syscall.Signal) Option {
	return func(o *option) {
		o.stopSignals = sigs
	}
}

// WithStopTimeout set stop timeout for graceful shutdown
//  if timeout occurs, running connections will be discard violently.
func WithStopTimeout(timeout time.Duration) Option {
	return func(o *option) {
		o.stopTimeout = timeout
	}
}

// WithWatchInterval set watch interval for worker checking master process state
func WithWatchInterval(timeout time.Duration) Option {
	return func(o *option) {
		o.watchInterval = timeout
	}
}

// address defines addr as well as its network type
type address struct {
	addr    string // ip:port, unix path
	network string // tcp, unix
}

type Server struct {
	opt      *option
	addrs    []address
	handlers []http.Handler
}

func NewServer(opts ...Option) *Server {
	option := &option{
		reloadSignals: defaultReloadSignals,
		stopSignals:   defaultStopSignals,
		watchInterval: defaultWatchInterval,
		stopTimeout:   defaultStopTimeout,
	}
	for _, opt := range opts {
		opt(option)
	}
	return &Server{
		addrs:    make([]address, 0),
		handlers: make([]http.Handler, 0),
		opt:      option,
	}
}

// Register an addr and its corresponding handler
// all (addr, handler) pair will be started with server.Run
func (s *Server) Register(addr string, handler http.Handler) {
	s.addrs = append(s.addrs, address{addr, "tcp"})
	s.handlers = append(s.handlers, handler)
}

// RegisterUnix register (addr, handler) on unix socket
func (s *Server) RegisterUnix(addr string, handler http.Handler) {
	s.addrs = append(s.addrs, address{addr, "unix"})
	s.handlers = append(s.handlers, handler)
}

// Run runs all register servers
func (s *Server) Run() error {
	if len(s.addrs) == 0 {
		return ErrNoServers
	}
	StartedAt = time.Now()
	if IsWorker() {
		worker := &worker{handlers: s.handlers, opt: s.opt, stopCh: make(chan struct{})}
		return worker.run()
	}
	master := &master{addrs: s.addrs, opt: s.opt, workerExit: make(chan error)}
	return master.run()
}

// Reload reload server gracefully
func (s *Server) Reload() error {
	ppid := os.Getppid()
	if IsWorker() && ppid != 1 && len(s.opt.reloadSignals) > 0 {
		return syscall.Kill(ppid, s.opt.reloadSignals[0])
	}

	// Reload called by user from outside usally in user's handler
	// which works on worker, master don't need to handle this
	return nil
}

// ListenAndServe starts server with (addr, handler)
func ListenAndServe(addr string, handler http.Handler) error {
	server := NewServer()
	server.Register(addr, handler)
	return server.Run()
}

func IsWorker() bool {
	return os.Getenv(EnvWorker) == ValWorker
}

func IsMaster() bool {
	return !IsWorker()
}
