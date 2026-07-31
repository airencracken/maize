package main

import (
	"bytes"
	"os"
	"path/filepath"
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
			args := []string{command}
			if command == "inspect" {
				args = append(args, "--help")
			}
			exitCode := run(args, &stdout, &stderr)
			if command == "inspect" {
				if exitCode != 0 || !strings.Contains(stderr.String(), "Usage of maize inspect") {
					t.Fatalf("inspect route: exit %d, stderr %q", exitCode, stderr.String())
				}
			} else {
				if exitCode != 2 || !strings.Contains(stderr.String(), "not implemented yet") {
					t.Fatalf("%s route: exit %d, stderr %q", command, exitCode, stderr.String())
				}
			}
		})
	}
}

func TestInspectRejectsInvalidArgumentsBeforeReadingHost(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"inspect", "--format", "yaml"},
		{"inspect", "--snapshot-consistency", "eventual"},
		{"inspect", "--repository", "missing-separator"},
		{"inspect", "--repository", "bad/name=/tmp"},
		{"inspect", "--repository", "same=/one", "--repository", "same=/two"},
		{"inspect", "positional"},
	}
	for _, args := range tests {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if exitCode := run(args, &stdout, &stderr); exitCode != 2 {
			t.Fatalf("%v exit code %d, want 2: %s", args, exitCode, stderr.String())
		}
	}
}

func TestInspectCommandRunsEndToEndAgainstAlternateRoot(t *testing.T) {
	t.Parallel()

	root := commandFixture(t)
	config := filepath.Join(root, "kernel.config")
	if err := os.WriteFile(config, []byte(
		"CONFIG_CGROUPS=y\nCONFIG_NAMESPACES=y\nCONFIG_SECCOMP=y\n"+
			"# CONFIG_SECCOMP_FILTER is not set\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(
		[]string{"inspect", "--root", root, "--config", config, "--format", "json"},
		&stdout, &stderr,
	)
	if exitCode != 0 || stderr.Len() != 0 {
		t.Fatalf("exit %d, stderr %q", exitCode, stderr.String())
	}
	for _, expected := range []string{
		`"schema": "maize.inspect/v2"`,
		`"symbol": "CONFIG_SECCOMP_FILTER"`,
		`"action": "enable"`,
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("output missing %q:\n%s", expected, stdout.String())
		}
	}
}

func commandFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sys"), 0o755); err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(root, "var", "db", "repos", "gentoo")
	profile := filepath.Join(repository, "profiles", "default", "linux", "amd64")
	commandWrite(t, filepath.Join(repository, "profiles", "base", "packages"), "*sys-apps/baselayout\n")
	commandWrite(t, filepath.Join(profile, "parent"), "../../../base\n")
	commandWrite(t, filepath.Join(profile, "make.defaults"), "ARCH=\"amd64\"\n")
	active := filepath.Join(root, "etc", "portage", "make.profile")
	if err := os.MkdirAll(filepath.Dir(active), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(profile, active); err != nil {
		t.Fatal(err)
	}
	commandWrite(t, filepath.Join(root, "etc", "portage", "make.conf"), "USE=\"\"\n")
	commandWrite(t, filepath.Join(root, "usr", "share", "portage", "config", "make.globals"), "USE=\"\"\n")
	commandWrite(t, filepath.Join(root, "var", "lib", "portage", "world"), "app-containers/docker\n")
	vdb := filepath.Join(root, "var", "db", "pkg", "app-containers", "docker-28.3.2")
	for name, value := range map[string]string{
		"CONTENTS": "", "EAPI": "8", "SLOT": "0", "repository": "gentoo",
		"IUSE": "seccomp apparmor", "USE": "seccomp",
	} {
		commandWrite(t, filepath.Join(vdb, name), value)
	}
	return root
}

func commandWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
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
