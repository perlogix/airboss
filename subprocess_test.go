package airboss

import (
	"fmt"
	"io/ioutil"
	"os"
	"syscall"
	"testing"
	"time"
)

var p *ProcessManager
var sub *Subprocess

var shell = `#!/bin/sh
echo <&0
while true; do
echo "hello!"
sleep 1
done
`

func TestMain(m *testing.M) {
	f, err := ioutil.TempFile(".", "script")
	if err != nil {
		panic(err)
	}
	fname := f.Name()
	_, err = f.WriteString(shell)
	if err != nil {
		panic(err)
	}
	err = f.Chmod(0755)
	if err != nil {
		panic(err)
	}
	f.Close()
	p = NewProcessManager()
	sub, err = p.Fork("./" + fname)
	if err != nil {
		panic(err)
	}
	sub.RestartPolicy = "always"
	sub.Stdin.WriteString("hello!")
	go func() {
		for {
			e := <-sub.Errors
			os.Stderr.WriteString(e.Error())
			time.Sleep(10 * time.Millisecond)
		}
	}()
	_, err = sub.Start()
	if err != nil {
		panic(err)
	}
	time.Sleep(10 * time.Second)
	rc := m.Run()
	os.Remove("./" + fname)
	errs := p.KillAll()
	for _, e := range errs {
		panic(e)
	}
	os.Exit(rc)
}

func TestList(t *testing.T) {
	l, err := p.List()
	if err != nil {
		t.Error(err)
	}
	if len(l) != 1 {
		t.Errorf("List does not contain running process")
	}
	fmt.Printf("%+v\n", l)
}

func TestSignal(t *testing.T) {
	err := sub.Signal(syscall.SIGHUP)
	if err != nil {
		t.Error(err)
	}
	err = sub.Signal(syscall.Signal(999))
	if err == nil {
		t.Error("no error on bad signal")
	}
}

func TestRestart(t *testing.T) {
	oldpid := sub.PID
	newpid, err := sub.Restart()
	if err != nil {
		t.Error(err)
	}
	if oldpid == newpid {
		t.Error("Subprocess PIDs are the same")
	}
}

func TestWatcher(t *testing.T) {
	err := sub.Signal(os.Kill)
	if err != nil {
		t.Error(err)
	}
}

func TestMetrics(t *testing.T) {
	m := sub.Metrics
	if m.CPU == 0.0 && m.Memory == 0.0 && m.Threads == 0 {
		t.Error(m)
	}
}

func TestOpenFiles(t *testing.T) {
	l := sub.OpenFiles
	fmt.Println(l)
}
