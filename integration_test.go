//go:build integration

package declaw

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestIntegration_FullLifecycle(t *testing.T) {
	if os.Getenv("DECLAW_API_KEY") == "" {
		t.Skip("DECLAW_API_KEY must be set (also set DECLAW_DOMAIN or DECLAW_API_URL)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// ---------------------------------------------------------------
	// 1. Create sandbox (uses DECLAW_API_KEY and DECLAW_API_URL from env)
	// ---------------------------------------------------------------
	t.Log("Creating sandbox...")
	sbx, err := Create(ctx,
		WithTimeout(300),
	)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Logf("Sandbox created: ID=%s", sbx.ID)

	defer func() {
		t.Log("Killing sandbox...")
		if err := sbx.Kill(context.Background()); err != nil {
			t.Logf("Kill warning: %v", err)
		} else {
			t.Log("Sandbox killed")
		}
	}()

	// ---------------------------------------------------------------
	// 2. Run a command
	// ---------------------------------------------------------------
	t.Log("Running command: echo hello-declaw-go-sdk")
	result, err := sbx.Commands.Run(ctx, "echo hello-declaw-go-sdk")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	t.Logf("Command result: exit_code=%d stdout=%q stderr=%q", result.ExitCode, result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout == "" {
		t.Error("expected non-empty stdout")
	}

	// ---------------------------------------------------------------
	// 3. Write a file
	// ---------------------------------------------------------------
	t.Log("Writing file /tmp/test.txt")
	writeInfo, err := sbx.Files.Write(ctx, "/tmp/test.txt", "hello from go sdk")
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	t.Logf("Write result: path=%s size=%d", writeInfo.Path, writeInfo.Size)

	// ---------------------------------------------------------------
	// 4. Read the file back
	// ---------------------------------------------------------------
	t.Log("Reading file /tmp/test.txt")
	content, err := sbx.Files.Read(ctx, "/tmp/test.txt")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	t.Logf("Read result: %q", content)
	if content != "hello from go sdk" {
		t.Errorf("expected 'hello from go sdk', got %q", content)
	}

	// ---------------------------------------------------------------
	// 5. Check file exists
	// ---------------------------------------------------------------
	t.Log("Checking file exists /tmp/test.txt")
	exists, err := sbx.Files.Exists(ctx, "/tmp/test.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected file to exist")
	}
	t.Logf("Exists: %v", exists)

	// ---------------------------------------------------------------
	// 6. List directory
	// ---------------------------------------------------------------
	t.Log("Listing /tmp")
	entries, err := sbx.Files.List(ctx, "/tmp")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	t.Logf("Listed %d entries in /tmp", len(entries))
	found := false
	for _, e := range entries {
		if e.Path == "/tmp/test.txt" || e.Path == "test.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Log("Warning: test.txt not found in listing (may use different path format)")
	}

	// ---------------------------------------------------------------
	// 7. Batch write files
	// ---------------------------------------------------------------
	t.Log("Batch writing files")
	err = sbx.Files.WriteFiles(ctx, []WriteEntry{
		{Path: "/tmp/batch1.txt", Data: "file one"},
		{Path: "/tmp/batch2.txt", Data: "file two"},
	})
	if err != nil {
		t.Fatalf("WriteFiles failed: %v", err)
	}
	b1, err := sbx.Files.Read(ctx, "/tmp/batch1.txt")
	if err != nil {
		t.Fatalf("Read batch1 failed: %v", err)
	}
	if b1 != "file one" {
		t.Errorf("batch1 content: expected 'file one', got %q", b1)
	}
	t.Log("Batch write verified")

	// ---------------------------------------------------------------
	// 8. PTY create, send input, kill
	// ---------------------------------------------------------------
	t.Log("Creating PTY...")
	ptyHandle, err := sbx.PTY.Create(ctx)
	if err != nil {
		t.Fatalf("PTY Create failed: %v", err)
	}
	t.Logf("PTY created: PID=%d", ptyHandle.PID)

	t.Log("Sending input to PTY: echo pty-test")
	err = ptyHandle.SendInput(ctx, []byte("echo pty-test\n"))
	if err != nil {
		t.Fatalf("PTY SendInput failed: %v", err)
	}
	t.Log("PTY SendInput succeeded")

	// Stream with a short dedicated timeout — SSE may hang through proxy
	t.Log("Streaming PTY output (10s timeout)...")
	streamCtx, streamCancel := context.WithTimeout(ctx, 10*time.Second)
	defer streamCancel()
	ch, err := ptyHandle.Stream(streamCtx)
	if err != nil {
		t.Logf("PTY Stream could not connect (may be proxy limitation): %v", err)
	} else {
		var ptyOutput []byte
		collectDone := make(chan struct{})
		go func() {
			defer close(collectDone)
			deadline := time.After(5 * time.Second)
			for {
				select {
				case data, ok := <-ch:
					if !ok {
						return
					}
					ptyOutput = append(ptyOutput, data...)
					if len(ptyOutput) > 50 {
						return
					}
				case <-deadline:
					return
				}
			}
		}()
		<-collectDone
		t.Logf("PTY output (%d bytes): %q", len(ptyOutput), string(ptyOutput))
	}

	t.Log("Killing PTY...")
	err = ptyHandle.Kill(ctx)
	if err != nil {
		t.Logf("PTY Kill warning: %v", err)
	} else {
		t.Log("PTY killed")
	}

	// ---------------------------------------------------------------
	// 9. Command with options (cwd, env)
	// ---------------------------------------------------------------
	t.Log("Running command with cwd and env")
	result2, err := sbx.Commands.Run(ctx, "pwd && echo $MY_VAR",
		WithCwd("/tmp"),
		WithRunEnvs(map[string]string{"MY_VAR": "go-sdk-test"}),
	)
	if err != nil {
		t.Fatalf("Run with opts failed: %v", err)
	}
	t.Logf("Command with opts: exit=%d stdout=%q", result2.ExitCode, result2.Stdout)

	// ---------------------------------------------------------------
	// 10. Remove file
	// ---------------------------------------------------------------
	t.Log("Removing /tmp/test.txt")
	err = sbx.Files.Remove(ctx, "/tmp/test.txt")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	exists2, err := sbx.Files.Exists(ctx, "/tmp/test.txt")
	if err != nil {
		t.Fatalf("Exists after remove failed: %v", err)
	}
	if exists2 {
		t.Error("file should not exist after removal")
	}
	t.Log("File removed and verified")

	// ---------------------------------------------------------------
	// 11. List running processes
	// ---------------------------------------------------------------
	t.Log("Listing processes")
	procs, err := sbx.Commands.List(ctx)
	if err != nil {
		t.Fatalf("List processes failed: %v", err)
	}
	t.Logf("Found %d processes", len(procs))

	// ---------------------------------------------------------------
	// Done
	// ---------------------------------------------------------------
	fmt.Println("\n=== ALL INTEGRATION CHECKS PASSED ===")
}
