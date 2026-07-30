package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpListsEveryCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code %d, want 0", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
	for _, command := range []string{"inspect", "generate", "migrate", "check", "impact", "observe"} {
		if !strings.Contains(stdout.String(), "maize "+command) {
			t.Errorf("help does not list %q", command)
		}
	}
}

func TestEveryDeclaredCommandExists(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"inspect", "generate", "migrate", "check", "impact", "observe"} {
		command := command
		t.Run(command, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := run([]string{command}, &stdout, &stderr)
			if exitCode != 2 {
				t.Fatalf("exit code %d, want 2 for unimplemented command", exitCode)
			}
			if !strings.Contains(stderr.String(), "not implemented yet") {
				t.Fatalf("unexpected stderr: %q", stderr.String())
			}
		})
	}
}

func TestUnknownCommandIsRejected(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"../../hostile"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("exit code %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
