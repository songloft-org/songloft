// Package mediaexec runs Songloft's FFmpeg operations through either the
// platform executables or the native library linked into iOS Bundle builds.
package mediaexec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"time"
)

type Command struct {
	Args           []string
	Stdout, Stderr io.Writer
	WaitDelay      time.Duration

	ctx     context.Context
	probe   bool
	process *exec.Cmd
	reader  *io.PipeReader
	writer  *io.PipeWriter
	result  chan error
}

func FFmpeg(ctx context.Context, path string, args ...string) *Command {
	return command(ctx, path, false, args)
}

func FFprobe(ctx context.Context, path string, args ...string) *Command {
	return command(ctx, path, true, args)
}

func command(ctx context.Context, path string, probe bool, args []string) *Command {
	cmd := &Command{Args: append([]string{path}, args...), ctx: ctx, probe: probe}
	if !Native {
		cmd.process = exec.CommandContext(ctx, path, args...)
	}
	return cmd
}

func (c *Command) Output() ([]byte, error) {
	if c.process != nil {
		c.configureProcess()
		return c.process.Output()
	}
	if c.Stdout != nil {
		return nil, errors.New("media stdout already set")
	}
	var stdout bytes.Buffer
	c.Stdout = &stdout
	err := c.Run()
	return stdout.Bytes(), err
}

func (c *Command) CombinedOutput() ([]byte, error) {
	if c.process != nil {
		c.configureProcess()
		return c.process.CombinedOutput()
	}
	if c.Stderr != nil || c.Stdout != nil {
		return nil, errors.New("media output already set")
	}
	var output bytes.Buffer
	c.Stdout, c.Stderr = &output, &output
	err := c.Run()
	return output.Bytes(), err
}

func (c *Command) StdoutPipe() (io.ReadCloser, error) {
	if c.process != nil {
		return c.process.StdoutPipe()
	}
	if c.Stdout != nil || c.result != nil || c.writer != nil {
		return nil, errors.New("media stdout already set")
	}
	c.reader, c.writer = io.Pipe()
	return c.reader, nil
}

func (c *Command) configureProcess() {
	if c.Stdout != nil {
		c.process.Stdout = c.Stdout
	}
	c.process.Stderr, c.process.WaitDelay = c.Stderr, c.WaitDelay
}

func (c *Command) Start() error {
	if c.process != nil {
		c.configureProcess()
		return c.process.Start()
	}
	if c.result != nil {
		return errors.New("media command already started")
	}
	if err := c.ctx.Err(); err != nil {
		if c.writer != nil {
			_ = c.writer.CloseWithError(err)
		}
		return err
	}
	c.result = make(chan error, 1)
	go func() {
		stdout := c.Stdout
		if c.writer != nil {
			stdout = c.writer
			stop := context.AfterFunc(c.ctx, func() { _ = c.writer.CloseWithError(c.ctx.Err()) })
			defer stop()
		}
		err := runNative(c.ctx, c.probe, c.Args[1:], stdout, c.Stderr)
		if c.writer != nil {
			// Match a subprocess pipe: the read end observes EOF, Wait reports errors.
			_ = c.writer.Close()
		}
		c.result <- err
	}()
	return nil
}

func (c *Command) Wait() error {
	if c.process != nil {
		return c.process.Wait()
	}
	if c.result == nil {
		return errors.New("media command not started")
	}
	err := <-c.result
	if c.reader != nil {
		_ = c.reader.Close()
	}
	return err
}

func (c *Command) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}
