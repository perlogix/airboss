package librun

import (
	"bytes"
	"os"
	"time"
)

// Subprocess type
type Subprocess struct {
	parent        *ProcessManager
	PID           int
	Done          bool
	RC            int
	Success       bool
	Duration      time.Duration // ms
	Stdin         bytes.Buffer
	Stdout        chan string
	Stderr        chan string
	Command       string
	Flags         []string
	Env           map[string]string
	CWD           string
	RestartPolicy string // always, on-failure, never
	OpenFiles     []File
	Children      []int
	Username      string
	Metrics       struct {
		CPU     float64
		Memory  float64
		Threads int32
	}
}

// Start func
func (s *Subprocess) Start() error {
	return nil
}

// Stop func
func (s *Subprocess) Stop() error {
	return nil
}

// Restart func
func (s *Subprocess) Restart() (int, error) {
	return 0, nil
}

// Signal func
func (s *Subprocess) Signal(signal os.Signal) error {
	return nil
}

func (s *Subprocess) watch() {
	return
}

func (s *Subprocess) log() {
	return
}
