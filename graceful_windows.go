// +build windows

package graceful

import (
	"net/http"
	"time"
)

// address defines addr as well as its network type
type address struct {
	addr    string // ip:port, unix path
	network string // tcp, unix
}

type option struct {
	watchInterval time.Duration
	stopTimeout   time.Duration
}

type Server struct {
	opt      *option
	addrs    []address
	handlers []http.Handler
}

func NewServer(opts ...option) *Server {
	panic("platform windows unsupported")
	return nil
}

func (s *Server) Register(addr string, handler http.Handler) {
}

func (s *Server) RegisterUnix(addr string, handler http.Handler) {
}

func (s *Server) Run() error {
	panic("platform windows unsupported")
	return nil
}

func IsMaster() bool {
	return true
}

func IsWorker() bool {
	return false
}

func ListenAndServe(addr string, handler http.Handler) error {
	panic("platform windows unsupported")
	return nil
}
