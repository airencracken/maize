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

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/app"
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
	configPath := flags.String("config", "", "existing Linux .config")
	format := flags.String("format", "text", "output format: text or json")
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

	paths := shared.DefaultSystemPaths(*root)
	if len(repositories) == 0 {
		paths.Repositories = []shared.RepositoryPath{{
			Name: "gentoo", Path: filepath.Join(filepath.Clean(*root), "var", "db", "repos", "gentoo"),
		}}
	} else {
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
	selectedConfig := *configPath
	if selectedConfig == "" {
		selectedConfig = filepath.Join(filepath.Clean(*root), "usr", "src", "linux", ".config")
	}
	file, err := os.Open(selectedConfig)
	if err != nil {
		fmt.Fprintf(stderr, "open kernel configuration: %v\n", err)
		return 1
	}
	defer file.Close()

	inspection, err := app.Inspect(context.Background(), paths, nil, selectedConfig, file)
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
