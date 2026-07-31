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
	"github.com/airencracken/maize/internal/terminal"
)

const usage = `maize generates and migrates optimized Gentoo kernel configurations

Usage:
  maize inspect [--root /] [--config PATH] [--format text|json]
  maize generate --kernel-tree PATH --output PATH [inspection options]
  maize migrate [--root /] [--format text|json]
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
	inspection, format, style, verbose, code := loadInspection("inspect", args, stdout, stderr)
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
		err = report.InspectionTextWithOptions(
			stdout, inspection, report.TextOptions{Style: style, Verbose: verbose},
		)
	}
	if err != nil {
		fmt.Fprintf(stderr, "write report: %v\n", err)
		return 1
	}
	return 0
}

func loadInspection(
	command string,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) (app.Inspection, string, terminal.Style, bool, int) {
	flags := flag.NewFlagSet("maize "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "/", "Gentoo installation root")
	configPath := flags.String("config", "", "kernel config; default discovers running, boot, then source config")
	sysfsPath := flags.String("sysfs", "", "sysfs root; default ROOT/sys")
	procfsPath := flags.String("procfs", "", "procfs root; default ROOT/proc")
	format := flags.String("format", "text", "output format: text or json")
	colorMode := flags.String("color", "auto", "color output: auto, always, or never")
	verbose := flags.Bool("verbose", false, "show all supporting and unresolved evidence")
	snapshotMode := flags.String("snapshot-consistency", "locked", "snapshot consistency: locked or stabilized")
	var repositories repositoryFlags
	flags.Var(&repositories, "repository", "repository NAME=PATH; may be repeated")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return app.Inspection{}, "", terminal.Style{}, false, -1
		}
		return app.Inspection{}, "", terminal.Style{}, false, 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "maize %s does not accept positional arguments\n", command)
		return app.Inspection{}, "", terminal.Style{}, false, 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "invalid format %q\n", *format)
		return app.Inspection{}, "", terminal.Style{}, false, 2
	}
	mode, err := terminal.ParseColorMode(*colorMode)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return app.Inspection{}, "", terminal.Style{}, false, 2
	}
	consistency := maizegentoo.SnapshotLocked
	if *snapshotMode == "stabilized" {
		consistency = maizegentoo.SnapshotStabilized
	} else if *snapshotMode != "locked" {
		fmt.Fprintf(stderr, "invalid snapshot consistency %q\n", *snapshotMode)
		return app.Inspection{}, "", terminal.Style{}, false, 2
	}

	paths := shared.DefaultSystemPaths(*root)
	if len(repositories) != 0 {
		seenRepositories := make(map[string]bool)
		for _, raw := range repositories {
			name, path, found := strings.Cut(raw, "=")
			if !found || !validRepositoryName(name) || path == "" || seenRepositories[name] {
				fmt.Fprintf(stderr, "invalid repository %q; want NAME=PATH\n", raw)
				return app.Inspection{}, "", terminal.Style{}, false, 2
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
		return app.Inspection{}, "", terminal.Style{}, false, 1
	}
	return inspection, *format, terminal.StyleForWriter(mode, stdout), *verbose, 0
}

func runGenerate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		_, _, _, _, code := loadInspection("generate", args, stdout, stderr)
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
	kernelTree, remaining, err := takeOption(remaining, "--kernel-tree")
	if err != nil || kernelTree == "" {
		fmt.Fprintln(stderr, "maize generate requires exactly one --kernel-tree PATH")
		return 2
	}
	inspection, _, style, _, code := loadInspection("generate", remaining, stdout, stderr)
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
	validation, err := kernel.ValidateTarget(context.Background(), kernelTree, candidate)
	if err != nil {
		fmt.Fprintf(stderr, "generate validation: %v\n", err)
		return 1
	}
	if err := app.ValidateRequiredRecommendations(validation, inspection.Recommendations); err != nil {
		fmt.Fprintf(stderr, "generate validation: %v\n", err)
		return 1
	}
	if err := fileutil.WriteAtomic(output, 0o644, validation.Config.Write); err != nil {
		fmt.Fprintf(stderr, "generate output: %v\n", err)
		return 1
	}
	fmt.Fprintf(
		stdout, "%s target-validated kernel configuration to %s\n",
		style.BoldGreen("wrote"), style.Cyan(output),
	)
	fmt.Fprintf(stdout, "target Kconfig resolved %d requested symbol changes\n", len(validation.Changes))
	return 0
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	inspection, _, style, verbose, code := loadInspection("check", args, stdout, stderr)
	if code == -1 {
		return 0
	}
	if code != 0 {
		return code
	}
	inspection.Recommendations = app.Unsatisfied(inspection, true)
	if err := report.InspectionTextWithOptions(
		stdout, inspection, report.TextOptions{Style: style, Verbose: verbose},
	); err != nil {
		fmt.Fprintf(stderr, "write check report: %v\n", err)
		return 1
	}
	if len(inspection.Recommendations) != 0 {
		return 3
	}
	if len(inspection.DynamicKernelPolicy) != 0 {
		return 4
	}
	return 0
}

func runImpact(args []string, stdout, stderr io.Writer) int {
	if !hasOption(args, "--config") && !(len(args) == 1 && (args[0] == "--help" || args[0] == "-h")) {
		fmt.Fprintln(stderr, "maize impact requires --config PATH")
		return 2
	}
	inspection, format, style, verbose, code := loadInspection("impact", args, stdout, stderr)
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
		err = report.InspectionTextWithOptions(
			stdout, inspection, report.TextOptions{Style: style, Verbose: verbose},
		)
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
	colorMode := flags.String("color", "auto", "color output: auto, always, or never")
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
	mode, err := terminal.ParseColorMode(*colorMode)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	style := terminal.StyleForWriter(mode, stdout)
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
	fmt.Fprintf(
		stdout, "%s %s hardware devices in %s\n",
		style.BoldGreen("recorded"), style.Cyan(fmt.Sprint(len(inventory.Devices))),
		style.Cyan(*output),
	)
	return 0
}

func runMigrate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("maize migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	oldKconfig := flags.String("old-kconfig", "", "old Kconfig artifact")
	newKconfig := flags.String("new-kconfig", "", "new Kconfig artifact")
	oldConfig := flags.String("old-config", "", "old kernel config")
	newConfig := flags.String("new-config", "", "new kernel config")
	root := flags.String("root", "/", "Gentoo installation root")
	procfs := flags.String("procfs", "", "procfs root; default ROOT/proc")
	format := flags.String("format", "text", "output format: text or json")
	colorMode := flags.String("color", "auto", "color output: auto, always, or never")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || (*format != "text" && *format != "json") {
		fmt.Fprintln(stderr, "maize migrate accepts no positional arguments")
		return 2
	}
	mode, err := terminal.ParseColorMode(*colorMode)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	artifactCount := 0
	for _, value := range []string{*oldKconfig, *newKconfig, *oldConfig, *newConfig} {
		if value != "" {
			artifactCount++
		}
	}
	if artifactCount != 0 && artifactCount != 4 {
		fmt.Fprintln(
			stderr,
			"maize migrate requires all four explicit old/new Kconfig and config paths, or none",
		)
		return 2
	}
	if artifactCount == 0 {
		return runDefaultMigrate(*root, *procfs, *format, mode, stdout, stderr)
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
		err = report.MigrationTextStyled(
			stdout, changes, terminal.StyleForWriter(mode, stdout),
		)
	}
	if err != nil {
		fmt.Fprintf(stderr, "write migration report: %v\n", err)
		return 1
	}
	return 0
}

func runDefaultMigrate(
	root string,
	procfs string,
	format string,
	mode terminal.ColorMode,
	stdout io.Writer,
	stderr io.Writer,
) int {
	ctx := context.Background()
	paths := kernel.DefaultConfigPaths(root)
	if procfs != "" {
		paths.ProcConfig = filepath.Join(procfs, "config.gz")
		paths.ProcRelease = filepath.Join(procfs, "sys", "kernel", "osrelease")
	}
	running, source, err := kernel.LoadConfig(ctx, paths)
	if err != nil {
		fmt.Fprintf(stderr, "migrate running kernel: %v\n", err)
		return 1
	}
	if source.RunningRelease == "" {
		fmt.Fprintln(stderr, "migrate: running kernel release is unavailable")
		return 1
	}
	inventory, err := kernel.DiscoverSourceTrees(root, source.RunningRelease)
	if err != nil {
		fmt.Fprintf(stderr, "migrate target discovery: %v\n", err)
		return 1
	}
	validated, err := kernel.ValidateTarget(ctx, inventory.Target.Path, running)
	if err != nil {
		fmt.Fprintf(stderr, "migrate target validation: %v\n", err)
		return 1
	}
	emptyCatalog, err := kernel.NewCatalog()
	if err != nil {
		fmt.Fprintf(stderr, "migrate: initialize Kconfig catalog: %v\n", err)
		return 1
	}
	changes := kernel.Compare(emptyCatalog, emptyCatalog, running, validated.Config)
	reportContext := report.MigrationContext{
		RunningRelease: source.RunningRelease,
		RunningConfig:  source.Path,
		TargetRelease:  inventory.Target.Release,
		TargetTree:     inventory.Target.Path,
	}
	if format == "json" {
		err = report.MigrationJSONWithContext(stdout, reportContext, changes)
	} else {
		err = report.MigrationTextWithContext(
			stdout, reportContext, changes, terminal.StyleForWriter(mode, stdout),
		)
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
