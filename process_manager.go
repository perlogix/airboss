package airboss

import (
	"bytes"
	"fmt"
	"os"
	"sync"
)

// ProcessManager manages multiple Subprocess objects
type ProcessManager struct {
	Procs map[int]*Subprocess
	lock  sync.Mutex
}

// NewProcessManager returns a pointer to a new ProcessManager struct
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		Procs: map[int]*Subprocess{},
		lock:  sync.Mutex{},
	}
}

// Fork creates a new Subprocess but *does not* start it. At this point the process
// is not yet managed and needs to be started to become "managed"
func (p *ProcessManager) Fork(command string, args ...string) (*Subprocess, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &Subprocess{
		parent:        p,
		Done:          false,
		Success:       false,
		Stdin:         bytes.NewBuffer([]byte{}),
		Stdout:        bytes.NewBuffer([]byte{}),
		Stderr:        bytes.NewBuffer([]byte{}),
		Errors:        make(chan error, 64),
		Command:       command,
		Args:          args,
		Env:           map[string]string{},
		CWD:           cwd,
		RestartPolicy: "never",
		OpenFiles:     []File{},
		Children:      []int32{},
		UID:           os.Getuid(),
		terminate:     make(chan bool),
		restarting:    false,
		Metrics: Metrics{
			CPU:     0.0,
			Memory:  0.0,
			Threads: 0,
		},
	}, nil
}

// Kill kills a subprocess with the given PID
func (p *ProcessManager) Kill(pid int) error {
	if _, ok := p.Procs[pid]; !ok {
		return fmt.Errorf("Process with PID %v does not exist", pid)
	}
	err := p.Procs[pid].Signal(os.Kill)
	if err != nil {
		return err
	}
	p.lock.Lock()
	delete(p.Procs, pid)
	p.lock.Unlock()
	return nil
}

// KillAll kills all subprocesses managed by the instance of ProcessManager
func (p *ProcessManager) KillAll(sig ...os.Signal) []error {
	signal := os.Kill
	if len(sig) > 0 {
		signal = sig[0]
	}
	var errs []error
	for _, v := range p.Procs {
		err := v.Signal(signal)
		if err != nil {
			errs = append(errs, err)
		}
		if err == nil {
			p.lock.Lock()
			delete(p.Procs, v.PID)
			p.lock.Unlock()
		}
	}
	return errs
}

// List lists all subprocesses managed by the ProcessManager
func (p *ProcessManager) List() ([]*Subprocess, error) {
	var l []*Subprocess
	for _, v := range p.Procs {
		l = append(l, v)
	}
	return l, nil
}

func (p *ProcessManager) processStart(pid int, proc *Subprocess) {
	p.lock.Lock()
	p.Procs[pid] = proc
	p.lock.Unlock()
}

func (p *ProcessManager) processStop(pid int) {
	p.lock.Lock()
	delete(p.Procs, pid)
	p.lock.Unlock()
}
