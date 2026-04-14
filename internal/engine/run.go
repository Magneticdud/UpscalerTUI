package engine

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"sync"
)

// RunResult is the outcome of a binary execution.
type RunResult struct {
	Success bool
	Lines   []string // combined stdout+stderr lines
}

// runBinary executes cmd and streams output lines to the provided channel.
// It uses two goroutines to read stdout and stderr in parallel (avoids OS pipe deadlock).
// The channel is not closed by this function; it sends all lines and returns.
func runBinary(ctx context.Context, logCh chan<- string, name string, args ...string) bool {
	cmd := exec.CommandContext(ctx, name, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return false
	}

	if err := cmd.Start(); err != nil {
		return false
	}

	// Two goroutines, one per pipe, both feed the same log channel.
	var wg sync.WaitGroup
	pipeReader := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MB buffer, no line truncation
		for scanner.Scan() {
			select {
			case logCh <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}

	wg.Add(2)
	go pipeReader(stdout)
	go pipeReader(stderr)
	wg.Wait()

	err = cmd.Wait()
	return err == nil
}

// newCommand creates an exec.Cmd using context; exported for jpeg2png.go usage.
func newCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
