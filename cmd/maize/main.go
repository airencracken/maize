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
	"github.com/airencracken/maize/internal/fileutil"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/report"
)

const usage = `maize generates and migrates optimized Gentoo kernel configurations

Usage:
  maize inspect [--root /] [--config PATH] [--format text|json]
  maize generate --output PATH [inspection options]
  maize migrate --old-kconfig PATH --new-kconfig PATH --old-config PATH --new-config PATH
  maize check [inspection options]
  maize impact --config PATH [inspection options]
  maize observe --output PATH [--root /]
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "inspect":
		return runInspect(args[1:], stdout, stderr)
	case "generate":
		return runGenerate(args[1:], stdout, stderr)
	case "migrate":
		return runMigrate(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "impact":
		return runImpact(args[1:], stdout, stderr)
	case "observe":
		return runObserve(args[1:], stdout, stderr)
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
	inspection, format, code := loadInspection("inspect", args, stderr)
	if code == -1 {
		return 0
	}
	if code != 0 {
		return code
	}
	var err error
	if format == "json" {
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

func loadInspection(command string, args []string, stderr io.Writer) (app.Inspection, string, int) {
	flags := flag.NewFlagSet("maize "+command, flag.ContinueOnError)
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
			return app.Inspection{}, "", -1
		}
		return app.Inspection{}, "", 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "maize %s does not accept positional arguments\n", command)
		return app.Inspection{}, "", 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "invalid format %q\n", *format)
		return app.Inspection{}, "", 2
	}
	consistency := maizegentoo.SnapshotLocked
	if *snapshotMode == "stabilized" {
		consistency = maizegentoo.SnapshotStabilized
	} else if *snapshotMode != "locked" {
		fmt.Fprintf(stderr, "invalid snapshot consistency %q\n", *snapshotMode)
		return app.Inspection{}, "", 2
	}

	paths := shared.DefaultSystemPaths(*root)
	if len(repositories) != 0 {
		seenRepositories := make(map[string]bool)
		for _, raw := range repositories {
			name, path, found := strings.Cut(raw, "=")
			if !found || !validRepositoryName(name) || path == "" || seenRepositories[name] {
				fmt.Fprintf(stderr, "invalid repository %q; want NAME=PATH\n", raw)
				return app.Inspection{}, "", 2
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
		fmt.Fprintf(stderr, "%s: %v\n", command, err)
		return app.Inspection{}, "", 1
	}
	return inspection, *format, 0
}

func runGenerate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		_, _, code := loadInspection("generate", args, stderr)
		if code == -1 {
			return 0
		}
		return code
	}
	output, remaining, err := takeOption(args, "--output")
	if err != nil || output == "" {
		fmt.Fprintln(stderr, "maize generate requires exactly one --output PATH")
		return 2
	}
	inspection, _, code := loadInspection("generate", remaining, stderr)
	if code == -1 {
		return 0
	}
	if code != 0 {
		return code
	}
	if len(inspection.DynamicKernelPolicy) != 0 {
		fmt.Fprintf(
			stderr,
			"generate: %d dynamic package kernel policies require operator review\n",
			len(inspection.DynamicKernelPolicy),
		)
		return 1
	}
	candidate, err := app.CandidateConfig(inspection)
	if err != nil {
		fmt.Fprintf(stderr, "generate: %v\n", err)
		return 1
	}
	if err := fileutil.WriteAtomic(output, 0o644, candidate.Write); err != nil {
		fmt.Fprintf(stderr, "generate output: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote candidate kernel configuration to %s\n", output)
	fmt.Fprintln(stdout, "target Kconfig validation is still required")
	return 0
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	inspection, _, code := loadInspection("check", args, stderr)
	if code == -1 {
		return 0
	}
	if code != 0 {
		return code
	}
	inspection.Recommendations = app.Unsatisfied(inspection, true)
	if err := report.InspectionText(stdout, inspection); err != nil {
		fmt.Fprintf(stderr, "write check report: %v\n", err)
		return 1
	}
	if len(inspection.Recommendations) != 0 {
		return 3
	}
	return 0
}

func runImpact(args []string, stdout, stderr io.Writer) int {
	if !hasOption(args, "--config") && !(len(args) == 1 && (args[0] == "--help" || args[0] == "-h")) {
		fmt.Fprintln(stderr, "maize impact requires --config PATH")
		return 2
	}
	inspection, format, code := loadInspection("impact", args, stderr)
	if code == -1 {
		return 0
	}
	if code != 0 {
		return code
	}
	inspection.Recommendations = app.Unsatisfied(inspection, false)
	var err error
	if format == "json" {
		err = report.InspectionJSON(stdout, inspection)
	} else {
		err = report.InspectionText(stdout, inspection)
	}
	if err != nil {
		fmt.Fprintf(stderr, "write impact report: %v\n", err)
		return 1
	}
	return 0
}

func runObserve(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("maize observe", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "/", "system root")
	sysfs := flags.String("sysfs", "", "sysfs root; default ROOT/sys")
	output := flags.String("output", "", "inventory output path")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *output == "" {
		fmt.Fprintln(stderr, "maize observe requires --output PATH and no positional arguments")
		return 2
	}
	paths := hardware.DefaultSystemPaths(*root)
	if *sysfs != "" {
		paths.Sys = *sysfs
	}
	inventory, err := hardware.Collect(context.Background(), paths, hardware.CollectOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "observe: %v\n", err)
		return 1
	}
	if err := fileutil.WriteAtomic(*output, 0o644, func(writer io.Writer) error {
		return report.HardwareJSON(writer, inventory)
	}); err != nil {
		fmt.Fprintf(stderr, "observe output: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "recorded %d hardware devices in %s\n", len(inventory.Devices), *output)
	return 0
}

func runMigrate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("maize migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	oldKconfig := flags.String("old-kconfig", "", "old Kconfig artifact")
	newKconfig := flags.String("new-kconfig", "", "new Kconfig artifact")
	oldConfig := flags.String("old-config", "", "old kernel config")
	newConfig := flags.String("new-config", "", "new kernel config")
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *oldKconfig == "" || *newKconfig == "" ||
		*oldConfig == "" || *newConfig == "" ||
		(*format != "text" && *format != "json") {
		fmt.Fprintln(stderr, "maize migrate requires old/new Kconfig and config paths")
		return 2
	}
	oldCatalog, err := readKconfig(*oldKconfig)
	if err != nil {
		fmt.Fprintf(stderr, "migrate: %v\n", err)
		return 1
	}
	newCatalog, err := readKconfig(*newKconfig)
	if err != nil {
		fmt.Fprintf(stderr, "migrate: %v\n", err)
		return 1
	}
	oldParsed, _, err := kernel.LoadConfig(context.Background(), kernel.ConfigPaths{Explicit: *oldConfig})
	if err != nil {
		fmt.Fprintf(stderr, "migrate: %v\n", err)
		return 1
	}
	newParsed, _, err := kernel.LoadConfig(context.Background(), kernel.ConfigPaths{Explicit: *newConfig})
	if err != nil {
		fmt.Fprintf(stderr, "migrate: %v\n", err)
		return 1
	}
	changes := kernel.Compare(oldCatalog, newCatalog, oldParsed, newParsed)
	if *format == "json" {
		err = report.MigrationJSON(stdout, changes)
	} else {
		err = report.MigrationText(stdout, changes)
	}
	if err != nil {
		fmt.Fprintf(stderr, "write migration report: %v\n", err)
		return 1
	}
	return 0
}

func readKconfig(path string) (kernel.Catalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return kernel.Catalog{}, err
	}
	defer file.Close()
	return kernel.ParseKconfig(path, file)
}

func takeOption(args []string, name string) (string, []string, error) {
	var value string
	found := false
	var remaining []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if strings.HasPrefix(arg, name+"=") {
			if found {
				return "", nil, fmt.Errorf("duplicate %s", name)
			}
			value = strings.TrimPrefix(arg, name+"=")
			found = true
			continue
		}
		if arg == name {
			if found || index+1 >= len(args) {
				return "", nil, fmt.Errorf("invalid %s", name)
			}
			value = args[index+1]
			found = true
			index++
			continue
		}
		remaining = append(remaining, arg)
	}
	return value, remaining, nil
}

func hasOption(args []string, name string) bool {
	for _, arg := range args {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
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
