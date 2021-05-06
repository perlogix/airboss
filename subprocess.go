package airboss

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"
)

// Subprocess contains all the methods and properties to
// manage a subprocess.
type Subprocess struct {
	parent *ProcessManager
	cmd    *exec.Cmd
	// PID of the subprocess
	PID int

	// Will return "true" if the process has exited
	Done bool

	// Return Code of the exited process
	RC int

	// Will return "true" if the return code is 0 (good exit)
	Success bool

	// Total lifetime of the process in miliseconds
	Duration time.Duration // ms

	// Time that the process was started
	StartTime time.Time

	// Time at the process was stopped
	StopTime time.Time

	// Any data written to *Subprocess.Stdin *before* *Subprocess.Start() is called
	// will be written to the process' stdin immediately after startup.
	// Data is treated as newline (\n) delimited.
	Stdin *bytes.Buffer

	// A buffer containing all of the output from the process' stdout
	Stdout *bytes.Buffer

	// A buffer containing all of the output from the process' stderr
	Stderr *bytes.Buffer

	// A channel on which any non-process errors will be sent
	// This is useful for logging errors without having to do dynamic
	// logging configuration or some other messy system
	Errors chan error

	// The command to be run
	Command string

	// Any arguments to be supplied to the command
	Args []string

	// Additional environment variables to be supplied to the process
	// in addition to those returned by os.Environ()
	Env map[string]string

	// Current working directory of the process
	CWD string

	// Process restart policy
	RestartPolicy string // always, on-failure, never

	// Slice of the files the process is holding open
	OpenFiles []File

	// PIDs of any child processes
	Children []int32

	// User ID of the user that spawned the subprocess
	UID int

	// Rudimentary metrics for CPU, Memory, and thread count
	Metrics Metrics

	terminate  chan bool
	restarting bool
	info       *process.Process
}

// Start starts the subprocess and populates struct fields
func (s *Subprocess) Start() (int, error) {
	s.StartTime = time.Now()
	c := exec.Command(s.Command, s.Args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var e []string
	for k, v := range s.Env {
		e = append(e, k+"="+v)
	}
	c.Env = append(os.Environ(), e...)
	s.cmd = c
	ready := make(chan bool)
	go s.log(ready)
	stdin, err := s.cmd.StdinPipe()
	if err != nil {
		return -1, err
	}
	defer stdin.Close()
	<-ready
	err = s.cmd.Start()
	if err != nil {
		s.Errors <- err
		return -1, err
	}
	s.PID = s.cmd.Process.Pid
	s.info, err = process.NewProcess(int32(s.PID))
	if err != nil {
		s.Errors <- err
		return s.PID, err
	}
	s.parent.processStart(s.PID, s)
	go s.watch()
	if err != nil {
		s.Errors <- err
		return s.PID, err
	}
	if s.Stdin.Len() != 0 {
		for s.Stdin.Len() != 0 {
			str, err := s.Stdin.ReadString('\n')
			if err != nil && err != io.EOF {
				s.Errors <- err
				return s.PID, err
			}
			_, err = io.WriteString(stdin, str)
			if err != nil {
				s.Errors <- err
				return s.PID, err
			}
		}
	}
	return s.PID, nil
}

// Stop stops a subprocess
func (s *Subprocess) Stop() error {
	s.terminate <- true
	s.terminate <- true
	err := s.cmd.Process.Kill()
	if err != nil {
		s.Errors <- err
		return err
	}
	s.Done = true
	s.RC = s.cmd.ProcessState.ExitCode()
	s.parent.processStop(s.PID)
	s.Success = func() bool {
		return s.RC == 0
	}()
	s.StopTime = time.Now()
	s.Duration = (s.StopTime.Sub(s.StartTime) + s.Duration) * time.Millisecond
	return nil
}

// Restart calls Stop() and Start() in sequence
func (s *Subprocess) Restart() (int, error) {
	s.restarting = true
	defer func() { s.restarting = false }()
	err := s.Stop()
	if err != nil {
		s.Errors <- err
		return -1, err
	}
	_, err = s.Start()
	if err != nil {
		s.Errors <- err
		return -1, err
	}
	return s.PID, nil
}

// Signal allows a user to send a specific signal to the subprocess
func (s *Subprocess) Signal(signal os.Signal) error {
	err := s.cmd.Process.Signal(signal)
	if err != nil {
		s.Errors <- err
		return err
	}

	return nil
}

func (s *Subprocess) watch() {
	t := time.NewTicker(10 * time.Millisecond)
	for range t.C {
		select {
		case <-s.terminate:
			return
		default:
			if s.restarting {
				continue
			}
			status, err := s.info.Status()
			if err != nil {
				s.Errors <- err
				continue
			}
			if status == "T" || status == "Z" {
				if s.RestartPolicy == "always" {
					_, err := s.Restart()
					if err != nil {
						s.Errors <- err
					}
				} else if s.RestartPolicy == "on-failure" && !s.cmd.ProcessState.Success() {
					_, err := s.Restart()
					if err != nil {
						s.Errors <- err
					}
				} else {
					return
				}
			}
			var m Metrics
			p := s.info
			cpu, err := p.CPUPercent()
			if err != nil {
				s.Errors <- err
				continue
			}
			m.CPU = cpu
			mem, err := p.MemoryPercent()
			if err != nil {
				s.Errors <- err
				continue
			}
			m.Memory = float64(mem)
			threads, err := p.NumThreads()
			if err != nil {
				s.Errors <- err
				continue
			}
			m.Threads = threads
			s.Metrics = m
			lsof, err := p.OpenFiles()
			if err != nil {
				s.Errors <- err
			}
			var of []File
			for _, f := range lsof {
				of = append(of, File{
					Path: f.Path,
					Fd:   f.Fd,
				})
			}
			s.OpenFiles = of
			childs, err := p.Children()
			if err != nil {
				s.Errors <- err
			}
			var children []int32
			for _, c := range childs {
				children = append(children, c.Pid)
			}
			s.Children = children
		}
	}
}

func (s *Subprocess) log(ready chan bool) {
	out, err := s.cmd.StdoutPipe()
	if err != nil {
		s.Errors <- err
		panic(err)
	}
	e, err := s.cmd.StderrPipe()
	if err != nil {
		s.Errors <- err
		panic(err)
	}
	defer out.Close()
	defer e.Close()
	ready <- true
	t := time.NewTicker(100 * time.Millisecond)
	for range t.C {
		select {
		case <-s.terminate:
			return
		default:
			var b []byte
			_, err := out.Read(b)
			if err != nil && err != io.EOF {
				s.Errors <- err
				continue
			}
			_, err = s.Stdout.Write(b)
			if err != nil {
				s.Errors <- err
				continue
			}
			b = []byte{}
			_, err = e.Read(b)
			if err != nil && err != io.EOF {
				s.Errors <- err
				continue
			}
			_, err = s.Stderr.Write(b)
			if err != nil && err != io.EOF {
				s.Errors <- err
				continue
			}
		}
	}
}
