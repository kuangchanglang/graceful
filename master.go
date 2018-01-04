package graceful

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"code.byted.org/gopkg/pkg/log"
)

type master struct {
	addrs      []string
	opt        *option
	extraFiles []*os.File
	worker     *os.Process
	mu         sync.Mutex
	ch         chan error
	workerNum  int32
}

func (m *master) run() error {
	m.mu.Lock()
	// init fds
	err := m.initFDs()
	if err != nil {
		return err
	}

	// fork worker
	p, err := m.forkWorker()
	if err != nil {
		return err
	}
	m.worker = p
	m.mu.Unlock()

	// wait for worker to exit
	go m.waitWorker()

	// wait signal
	m.waitSignal()
	return nil
}

func (m *master) waitWorker() {
	for {
		select {
		case <-m.ch:
			atomic.AddInt32(&m.workerNum, -1)
			if m.workerNum <= 0 { // all workers exit
				m.stop()
			}
		}
	}
}

func (m *master) waitSignal() {
	ch := make(chan os.Signal)
	sigs := make([]os.Signal, 0, len(m.opt.reloadSignals)+len(m.opt.stopSignals))
	sigs = append(sigs, m.opt.reloadSignals...)
	sigs = append(sigs, m.opt.stopSignals...)
	signal.Notify(ch, sigs...)
	for {
		sig := <-ch
		log.Infof("got signal: %v", sig)
		for _, s := range m.opt.reloadSignals {
			if s == sig {
				m.reload()
				break
			}
		}
		for _, s := range m.opt.stopSignals {
			if s == sig {
				m.stop()
				break
			}
		}
	}
}

func (m *master) reload() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// start new worker
	p, err := m.forkWorker()
	if err != nil {
		return
	}
	// stop old worker
	m.worker.Signal(syscall.SIGKILL)
	m.worker.Wait() // avoid child proccess being zombie
	m.worker = p
}

func (m *master) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	os.Exit(0)
}

func (m *master) initFDs() error {
	m.extraFiles = make([]*os.File, 0, len(m.addrs))
	for _, addr := range m.addrs {
		a, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return fmt.Errorf("Invalid address %s (%s)", addr, err)
		}
		l, err := net.ListenTCP("tcp", a)
		if err != nil {
			return err
		}
		f, err := l.File()
		if err != nil {
			return fmt.Errorf("Failed to retreive fd for: %s (%s)", addr, err)
		}
		if err := l.Close(); err != nil {
			return fmt.Errorf("Failed to close listener for: %s (%s)", addr, err)
		}
		m.extraFiles = append(m.extraFiles, f)
	}
	return nil
}

func (m *master) forkWorker() (*os.Process, error) {
	path := os.Args[0]
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}

	env := append(os.Environ(), fmt.Sprintf("%s=%s", EnvWorker, ValWorker), fmt.Sprintf("%s=%d", EnvNumFD, len(m.extraFiles)))
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = m.extraFiles
	cmd.Env = env
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	atomic.AddInt32(&m.workerNum, 1)
	go func() {
		m.ch <- cmd.Wait()
	}()
	return cmd.Process, nil
}
