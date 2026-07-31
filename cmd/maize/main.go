package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/app"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/report"
)

const usage = `maize generates and migrates optimized Gentoo kernel configurations

Usage:
  maize inspect [--root /] [--config PATH] [--format text|json]
  maize generate
  maize migrate
  maize check
  maize impact
  maize observe
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "inspect":
		return runInspect(args[1:], stdout, stderr)
	case "generate", "migrate", "check", "impact", "observe":
		fmt.Fprintf(stderr, "maize %s is not implemented yet\n", args[0])
		return 2
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

type repositoryFlags []string

func (values *repositoryFlags) String() string {
	return strings.Join(*values, ",")
}

func (values *repositoryFlags) Set(value string) error {
	if value == "" {
		return fmt.Errorf("repository cannot be empty")
	}
	*values = append(*values, value)
	return nil
}

func runInspect(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("maize inspect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "/", "Gentoo installation root")
	configPath := flags.String("config", "", "kernel config; default discovers running, boot, then source config")
	sysfsPath := flags.String("sysfs", "", "sysfs root; default ROOT/sys")
	procfsPath := flags.String("procfs", "", "procfs root; default ROOT/proc")
	format := flags.String("format", "text", "output format: text or json")
	snapshotMode := flags.String("snapshot-consistency", "locked", "snapshot consistency: locked or stabilized")
	var repositories repositoryFlags
	flags.Var(&repositories, "repository", "repository NAME=PATH; may be repeated")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "maize inspect does not accept positional arguments")
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "invalid format %q\n", *format)
		return 2
	}
	consistency := maizegentoo.SnapshotLocked
	if *snapshotMode == "stabilized" {
		consistency = maizegentoo.SnapshotStabilized
	} else if *snapshotMode != "locked" {
		fmt.Fprintf(stderr, "invalid snapshot consistency %q\n", *snapshotMode)
		return 2
	}

	paths := shared.DefaultSystemPaths(*root)
	if len(repositories) != 0 {
		seenRepositories := make(map[string]bool)
		for _, raw := range repositories {
			name, path, found := strings.Cut(raw, "=")
			if !found || !validRepositoryName(name) || path == "" || seenRepositories[name] {
				fmt.Fprintf(stderr, "invalid repository %q; want NAME=PATH\n", raw)
				return 2
			}
			seenRepositories[name] = true
			paths.Repositories = append(paths.Repositories, shared.RepositoryPath{Name: name, Path: path})
		}
	}
	configPaths := kernel.DefaultConfigPaths(*root)
	configPaths.Explicit = *configPath
	hardwarePaths := hardware.DefaultSystemPaths(*root)
	if *sysfsPath != "" {
		hardwarePaths.Sys = *sysfsPath
	}
	if *procfsPath != "" {
		configPaths.ProcConfig = filepath.Join(*procfsPath, "config.gz")
		configPaths.ProcRelease = filepath.Join(*procfsPath, "sys", "kernel", "osrelease")
	}

	inspection, err := app.InspectSystem(
		context.Background(), paths, nil, configPaths, hardwarePaths, time.Time{}, consistency,
	)
	if err != nil {
		fmt.Fprintf(stderr, "inspect: %v\n", err)
		return 1
	}
	if *format == "json" {
		err = report.InspectionJSON(stdout, inspection)
	} else {
		err = report.InspectionText(stdout, inspection)
	}
	if err != nil {
		fmt.Fprintf(stderr, "write report: %v\n", err)
		return 1
	}
	return 0
}

func validRepositoryName(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
