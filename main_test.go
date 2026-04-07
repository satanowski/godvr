package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout captures everything written to os.Stdout during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}

func TestPrintUsageContainsCommands(t *testing.T) {
	// Set os.Args[0] so the binary name is predictable.
	oldArgs := os.Args
	os.Args = []string{"godvr-test"}
	defer func() { os.Args = oldArgs }()

	output := captureStdout(t, printUsage)

	commands := []string{"e", "s", "t", "p", "r", "i", "v", "w", "x"}
	for _, cmd := range commands {
		if !strings.Contains(output, cmd) {
			t.Errorf("printUsage() output missing command %q", cmd)
		}
	}
}

func TestPrintUsageContainsEnvVars(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"godvr-test"}
	defer func() { os.Args = oldArgs }()

	output := captureStdout(t, printUsage)

	envVars := []string{
		"DVR_PG_CONN",
		"DVR_DVB_FRONT",
		"DVR_DVB_LNA",
		"DVR_DVB_DONGLES_COUNT",
		"DVR_CHANNELS_FILE",
		"DVR_REC_DIR",
		"DVR_REC_DONE_DIR",
		"DVR_TIME_SHIFT_BEFORE",
		"DVR_TIME_SHIFT_AFTER",
		"DVR_INTERVAL_SEC",
	}
	for _, env := range envVars {
		if !strings.Contains(output, env) {
			t.Errorf("printUsage() output missing env var %q", env)
		}
	}
}

func TestPrintUsageShowsBinaryName(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"/usr/local/bin/mydvr"}
	defer func() { os.Args = oldArgs }()

	output := captureStdout(t, printUsage)

	if !strings.Contains(output, "mydvr") {
		t.Errorf("printUsage() should show binary basename 'mydvr', got:\n%s", output)
	}
	// Should not contain the full path.
	if strings.Contains(output, "/usr/local/bin/mydvr") {
		t.Errorf("printUsage() should show basename only, not full path")
	}
}

func TestPrintUsageMasksPGConn(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"godvr-test"}
	defer func() { os.Args = oldArgs }()

	t.Setenv("DVR_PG_CONN", "postgres://user:secret@localhost/db")

	output := captureStdout(t, printUsage)

	if strings.Contains(output, "secret") {
		t.Error("printUsage() should mask DVR_PG_CONN value, but it leaked the secret")
	}
	if !strings.Contains(output, "****") {
		t.Error("printUsage() should show masked value '****' for DVR_PG_CONN")
	}
}

func TestPrintUsageShowsEnvValue(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"godvr-test"}
	defer func() { os.Args = oldArgs }()

	t.Setenv("DVR_REC_DIR", "/my/recordings")

	output := captureStdout(t, printUsage)

	if !strings.Contains(output, "/my/recordings") {
		t.Errorf("printUsage() should display current DVR_REC_DIR value, got:\n%s", output)
	}
}

func TestPrintUsageShowsDefault(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"godvr-test"}
	defer func() { os.Args = oldArgs }()

	// Ensure DVR_INTERVAL_SEC is unset so default is shown.
	t.Setenv("DVR_INTERVAL_SEC", "")

	output := captureStdout(t, printUsage)

	if !strings.Contains(output, "default:") {
		t.Errorf("printUsage() should show default indicator for unset env vars, got:\n%s", output)
	}
}
