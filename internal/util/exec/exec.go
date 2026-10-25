package exec

import (
	"context"
	"os/exec"
	"sync"
)

// proc tracks one started process; done closes after the process is reaped.
type proc struct {
	done chan struct{}
	err  error
}

// Cmd wraps exec.Cmd with a restartable, safely waitable process.
type Cmd struct {
	*exec.Cmd

	p  *proc
	mu sync.Mutex
}

// Start starts the process and reaps it in the background.
func (cmd *Cmd) Start() error {
	cmd.mu.Lock()
	defer cmd.mu.Unlock()
	if err := cmd.Cmd.Start(); err != nil {
		return err
	}
	p := &proc{done: make(chan struct{})}
	cmd.p = p
	inner := cmd.Cmd
	go func() {
		p.err = inner.Wait()
		close(p.done)
	}()
	return nil
}

// Waiter binds to the current process and returns a func that blocks until it
// exits. Safe to call from many goroutines, and unaffected by later restarts.
func (cmd *Cmd) Waiter() func() error {
	cmd.mu.Lock()
	p := cmd.p
	cmd.mu.Unlock()
	return func() error {
		if p == nil {
			return nil
		}
		<-p.done
		return p.err
	}
}

// Done is closed when the current process exits.
func (cmd *Cmd) Done() <-chan struct{} {
	cmd.mu.Lock()
	defer cmd.mu.Unlock()
	if cmd.p == nil {
		return nil
	}
	return cmd.p.done
}

// Wait blocks until the current process exits.
func (cmd *Cmd) Wait() error { return cmd.Waiter()() }

// Restart kills the process, waits for it to exit so its ports are free, then
// starts a fresh one.
func (cmd *Cmd) Restart(ctx context.Context) error {
	wait := cmd.Waiter()
	if cmd.Process != nil {
		// The process may already have exited; Wait below reaps it either way.
		_ = cmd.Process.Kill()
		_ = wait()
	}
	newCmd := exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)
	newCmd.Dir = cmd.Dir
	newCmd.Env = cmd.Env
	newCmd.Stdin = cmd.Stdin
	newCmd.Stdout = cmd.Stdout
	newCmd.Stderr = cmd.Stderr
	cmd.mu.Lock()
	cmd.Cmd = newCmd
	cmd.mu.Unlock()
	return cmd.Start()
}

// CommandContext returns a Cmd that runs name with args under ctx.
func CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	return &Cmd{Cmd: exec.CommandContext(ctx, name, args...)}
}
