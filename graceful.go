package graceful

import (
	"net/http"
	"os"
	"syscall"
	"time"
)

const (
	EnvWorker = "GRACEFUL_WORKER"
	EnvNumFD  = "GRACEFUL_NUMFD"
	ValWorker = "1"
)

var (
	defaultWatchInterval = time.Second
	defaultStopTimeout   = 20 * time.Second
	defaultReloadSignals = []os.Signal{syscall.SIGHUP, syscall.SIGUSR1}
	defaultStopSignals   = []os.Signal{syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT}
)

type option struct {
	reloadSignals []os.Signal
	stopSignals   []os.Signal
	watchInterval time.Duration
	stopTimeout   time.Duration
}

type Option func(o *option)

// WithReloadSignals set reload signals, otherwise, default ones are used
func WithReloadSignals(sigs []os.Signal) Option {
	return func(o *option) {
		o.reloadSignals = sigs
	}
}

// WithStopSignals set stop signals, otherwise, default ones are used
func WithStopSignals(sigs []os.Signal) Option {
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

type Server struct {
	opt      *option
	addrs    []string
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
		addrs:    make([]string, 0),
		handlers: make([]http.Handler, 0),
		opt:      option,
	}
}

// Register an addr and its corresponding handler
// all (addr, handler) pair will be started with server.Run
func (s *Server) Register(addr string, handler http.Handler) {
	s.addrs = append(s.addrs, addr)
	s.handlers = append(s.handlers, handler)
}

// Run runs all register servers
func (s *Server) Run() error {
	if len(s.addrs) == 0 {
		return ErrNoServers
	}
	if os.Getenv(EnvWorker) == ValWorker {
		worker := &worker{handlers: s.handlers, opt: s.opt}
		return worker.run()
	}
	master := &master{addrs: s.addrs, opt: s.opt, ch: make(chan error)}
	return master.run()
}

// ListenAndServe starts server with (addr, handler)
func ListenAndServe(addr string, handler http.Handler) error {
	server := NewServer()
	server.Register(addr, handler)
	return server.Run()
}
