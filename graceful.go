package graceful

import (
	"net/http"
	"os"
	"syscall"
)

const (
	EnvWorker = "GRACEFUL_WORKER"
	EnvNumFD  = "GRACEFUL_NUMFD"
	ValWorker = "1"
)

type option struct {
	reloadSignals []os.Signal
	stopSignals   []os.Signal
}

type Option func(o *option)

func WithReloadSignals(sigs []os.Signal) Option {
	return func(o *option) {
		o.reloadSignals = sigs
	}
}
func WithStopSignals(sigs []os.Signal) Option {
	return func(o *option) {
		o.stopSignals = sigs
	}
}

type Server struct {
	opt      *option
	addrs    []string
	handlers []http.Handler
}

func NewServer(opts ...Option) *Server {
	option := &option{
		reloadSignals: []os.Signal{syscall.SIGHUP, syscall.SIGUSR1},
		stopSignals:   []os.Signal{syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT},
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

func (s *Server) Register(addr string, handler http.Handler) {
	s.addrs = append(s.addrs, addr)
	s.handlers = append(s.handlers, handler)
}

func (s *Server) Run() error {
	if os.Getenv(EnvWorker) == ValWorker {
		worker := &worker{handlers: s.handlers}
		return worker.run()
	}
	master := &master{addrs: s.addrs, opt: s.opt, ch: make(chan error)}
	return master.run()
}

func ListenAndServe(addr string, handler http.Handler) error {
	server := NewServer()
	server.Register(addr, handler)
	return server.Run()
}
