//go:build linux

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"bindwitness/internal/witness"
)

func TestCLISignalHelper(t *testing.T) {
	if os.Getenv("BW_CLI_SIGNAL_HELPER") != "1" {
		return
	}
	os.Args = []string{"bindwitness", "capture", "--config", os.Getenv("BW_SIGNAL_CONFIG"), "--report", os.Getenv("BW_SIGNAL_REPORT")}
	main()
}

func TestCLISignalCleansWorkload(t *testing.T) {
	root := t.TempDir()
	if b, err := exec.Command("sh", "../../scripts/build-fixtures.sh", root).CombinedOutput(); err != nil {
		t.Fatalf("build fixtures: %s: %v", b, err)
	}
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			dir := t.TempDir()
			pidFile := filepath.Join(dir, "workload.pid")
			config, report := filepath.Join(dir, "config.json"), filepath.Join(dir, "report.json")
			var c witness.Config
			if err := witness.ReadJSON("../../examples/native.json", &c); err != nil {
				t.Fatal(err)
			}
			c.WorkingDirectory = root
			c.Roots["fixtures"] = root
			c.Command[4] = "signal"
			c.Environment = map[string]string{"BW_PID_FILE": pidFile}
			c.Deadline = "10s"
			if err := witness.WriteJSON(config, c); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCLISignalHelper$")
			cmd.Env = append(os.Environ(), "BW_CLI_SIGNAL_HELPER=1", "BW_SIGNAL_CONFIG="+config, "BW_SIGNAL_REPORT="+report)
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Kill()
			pid := 0
			defer func() {
				if pid > 0 {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			}()
			until := time.Now().Add(5 * time.Second)
			for time.Now().Before(until) {
				if b, err := os.ReadFile(pidFile); err == nil {
					pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
					if pid > 0 {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid <= 0 {
				t.Fatal("workload did not reach the signal checkpoint")
			}
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			err := cmd.Wait()
			if cmd.ProcessState.ExitCode() != 2 || time.Since(start) > 3*time.Second {
				t.Fatalf("cancelled capture: %v, %s", err, output.String())
			}
			if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
				t.Fatalf("workload %d survived CLI cancellation: %v", pid, err)
			}
			pid = 0 // Reaped by the CLI; do not signal a subsequently reused PID.
			var o witness.Observation
			if err := witness.ReadJSON(report, &o); err != nil {
				t.Fatal(err)
			}
			if o.Workload.DeadlineExceeded || o.Capture.Complete || o.Workload.ExitCode == nil || *o.Workload.ExitCode != -1 {
				t.Fatal(o.Workload, o.Capture)
			}
			found := false
			for _, issue := range o.Capture.Issues {
				found = found || issue.ID == "WORKLOAD_CANCELLED"
			}
			if !found {
				t.Fatalf("missing cancellation finding: %+v", o.Capture)
			}
		})
	}
}
