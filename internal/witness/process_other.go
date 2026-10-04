//go:build !linux

package witness

import (
	"context"
	"os/exec"
	"time"
)

func captureContext(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

func prepareProcess(cmd *exec.Cmd) {}
func finishProcess(cmd *exec.Cmd)  {}
