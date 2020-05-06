package librun

// ProcessManager type
type ProcessManager struct {
	Procs map[int]*Subprocess
}

// NewProcessManager function
func NewProcessManager() *ProcessManager {
	return &ProcessManager{}
}

// Fork function
func (p *ProcessManager) Fork() (*Subprocess, error) {
	return &Subprocess{}, nil
}

// Kill function
func (p *ProcessManager) Kill(pid int) error {
	return nil
}

// List function
func (p *ProcessManager) List() ([]*Subprocess, error) {
	return []*Subprocess{}, nil
}
