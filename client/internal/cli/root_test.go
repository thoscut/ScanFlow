package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestVersionCommandUsesConfiguredVersion(t *testing.T) {
	originalVersion := appVersion
	appVersion = "v9.9.9"
	t.Cleanup(func() {
		appVersion = originalVersion
	})

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	cmd := &cobra.Command{
		Use: "version",
		Run: versionCmd.Run,
	}

	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("execute version command: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	var got bytes.Buffer
	if _, err := io.Copy(&got, r); err != nil {
		t.Fatalf("read output: %v", err)
	}

	if got.String() != "scanflow client v9.9.9\n" {
		t.Fatalf("unexpected version output: %q", got.String())
	}
}
