// +build !windows

package graceful

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

type master struct {
	addrs      []address  // addrs to be listen, master use them to get file fds
	opt        *option    // option config
	extraFiles []*os.File // listeners fds communicated between master and worker
	workerPid  int        // worker proccess
	workerExit chan error // channel waiting for worker.Wait()

	// if livingWorkerNum could be:
	//  0: all workers exit,
	//  1: worker running,
	//  2: reloading, new worker is up and old worker about to exit
	//
	// if livingWorkerNum down to 0, we kill master as well
	livingWorkerNum int32
	sync.Mutex
}

func (m *master) run() error {
	m.Lock()
	// init fds
	err := m.initFDs()
	if err != nil {
		m.Unlock()
		return err
	}

	// fork worker
	pid, err := m.forkWorker()
	if err != nil {
		m.Unlock()
		return err
	}
	m.workerPid = pid
	m.Unlock()

	// wait signal
	m.waitSignal()
	return nil
}

func (m *master) waitSignal() {
	ch := make(chan os.Signal)
	sigs := make([]os.Signal, 0, len(m.opt.reloadSignals)+len(m.opt.stopSignals))
	for _, s := range m.opt.reloadSignals {
		sigs = append(sigs, s)
	}
	for _, s := range m.opt.stopSignals {
		sigs = append(sigs, s)
	}
	signal.Notify(ch, sigs...)
	for {
		var sig os.Signal
		select {
		case err := <-m.workerExit:
			if _, ok := err.(*exec.ExitError); ok {
				log.Printf("worker exit with error: %+v, master is going to shutdown.", err)
				m.stop()
				return
			}
			atomic.AddInt32(&m.livingWorkerNum, -1)
			if m.livingWorkerNum <= 0 {
				log.Printf("all workers exit, master is going to shutdown.")
				m.stop()
				return
			}
			continue
		case sig = <-ch:
			log.Printf("master got signal: %v\n", sig)
		}

		for _, s := range m.opt.reloadSignals {
			if s == sig {
				m.reload()
				break
			}
		}
		for _, s := range m.opt.stopSignals {
			if s == sig {
				m.stop()
				return
			}
		}
	}
}

func (m *master) reload() {
	m.Lock()
	defer m.Unlock()

	// start new worker
	p, err := m.forkWorker()
	if err != nil {
		log.Printf("[reload] fork worker error: %v\n", err)
		return
	}

	m.workerPid = p
}

func (m *master) stop() {
	// todo
}

// initFDs inits all registered addrs
func (m *master) initFDs() error {
	m.extraFiles = make([]*os.File, 0, len(m.addrs))
	for _, addr := range m.addrs {
		f, err := m.listen(addr)
		if err != nil {
			return fmt.Errorf("failed to listen on addr: %s, err: %v", addr, err)
		}

		m.extraFiles = append(m.extraFiles, f)
	}
	return nil
}

// listen return listening file for given addr, tcp and unix socket supported
func (m *master) listen(addr address) (*os.File, error) {
	if addr.network == "tcp" {
		a, err := net.ResolveTCPAddr("tcp", addr.addr)
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", a)
		if err != nil {
			return nil, err
		}
		f, err := l.File()
		if err != nil {
			return nil, err
		}
		if err := l.Close(); err != nil {
			return nil, err
		}
		return f, nil
	}

	if addr.network == "unix" {
		a, err := net.ResolveUnixAddr("unix", addr.addr)
		if err != nil {
			return nil, err
		}
		syscall.Unlink(addr.addr)
		l, err := net.ListenUnix("unix", a)
		if err != nil {
			return nil, err
		}
		f, err := l.File()
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	return nil, fmt.Errorf("unknown network: %v", addr.network)
}

func (m *master) forkWorker() (int, error) {
	path := os.Args[0]
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}

	env := append(os.Environ(), fmt.Sprintf("%s=%s", EnvWorker, ValWorker), fmt.Sprintf("%s=%d", EnvNumFD, len(m.extraFiles)), fmt.Sprintf("%s=%d", EnvParentPid, os.Getpid()), fmt.Sprintf("%s=%d", EnvOldWorkerPid, m.workerPid))

	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = m.extraFiles
	cmd.Env = env
	err := cmd.Start()
	if err != nil {
		return 0, err
	}
	atomic.AddInt32(&m.livingWorkerNum, 1)
	go func() {
		m.workerExit <- cmd.Wait()
	}()
	return cmd.Process.Pid, nil
}
