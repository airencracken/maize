package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/app"
	"github.com/airencracken/maize/internal/fileutil"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
	"github.com/airencracken/maize/internal/hardware"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/recommend"
	"github.com/airencracken/maize/internal/report"
	"github.com/airencracken/maize/internal/terminal"
)

const usage = `maize generates and migrates optimized Gentoo kernel configurations

Usage:
  maize inspect [--root /] [--config PATH] [--format text|json]
  maize generate --kernel-tree PATH --output PATH [--experimental-best-guess|--experimental-minimize] [inspection options]
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
	flags.Usage = func() { writeInspectionHelp(flags, command, stderr) }
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
		hardwarePaths.Proc = *procfsPath
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
	if hasHelpSwitch(args) {
		writeGenerateHelp(stderr)
		return 0
	}
	output, remaining, err := takeOption(args, "--output")
	if err != nil {
		fmt.Fprintln(stderr, "maize generate accepts --output PATH at most once")
		return 2
	}
	if output == "" {
		output = "maize.config"
	}
	kernelTree, remaining, err := takeOption(remaining, "--kernel-tree")
	if err != nil {
		fmt.Fprintln(stderr, "maize generate accepts --kernel-tree PATH at most once")
		return 2
	}
	experimental, remaining, err := takeSwitch(remaining, "--experimental-best-guess")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	minimize, remaining, err := takeSwitch(remaining, "--experimental-minimize")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if experimental && minimize {
		fmt.Fprintln(stderr, "--experimental-best-guess and --experimental-minimize are mutually exclusive")
		return 2
	}
	inspection, _, style, verbose, code := loadInspection("generate", remaining, stdout, stderr)
	if code == -1 {
		return 0
	}
	if code != 0 {
		return code
	}
	if kernelTree == "" {
		root := inspectionRoot(remaining)
		inventory, discoverErr := kernel.DiscoverSourceTrees(root, "")
		if discoverErr != nil {
			fmt.Fprintf(stderr, "generate target discovery: %v\n", discoverErr)
			return 1
		}
		kernelTree = inventory.Target.Path
		fmt.Fprintf(stdout, "selected newest installed kernel source %s (%s)\n", style.Cyan(kernelTree), inventory.Target.Release)
	}
	if len(inspection.DynamicKernelPolicy) != 0 {
		writeDynamicPolicyFailure(stderr, inspection.DynamicKernelPolicy, verbose)
		return 1
	}
	candidate, err := app.CandidateConfig(inspection)
	if err != nil {
		fmt.Fprintf(stderr, "generate: %v\n", err)
		return 1
	}
	optimizedModules := 0
	observedModules := 0
	if experimental || minimize {
		protected := make([]kernel.Symbol, 0, len(inspection.Recommendations))
		for _, recommendation := range inspection.Recommendations {
			protected = append(protected, recommendation.Symbol)
		}
		modules := append([]string(nil), inspection.Hardware.LoadedModules...)
		for _, device := range inspection.Hardware.Devices {
			if device.Presence == hardware.Present {
				modules = append(modules, device.Modules...)
			}
		}
		strategy := kernel.OptimizeBestGuess
		if minimize {
			strategy = kernel.OptimizeMinimize
		}
		optimization, optimizeErr := kernel.ExperimentalOptimize(
			context.Background(), kernelTree, candidate, protected, modules, strategy,
		)
		if optimizeErr != nil {
			fmt.Fprintf(stderr, "generate experimental optimization: %v\n", optimizeErr)
			return 1
		}
		candidate = optimization.Config
		optimizedModules = optimization.DisabledModules
		observedModules = len(optimization.ObservedModules)
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
	output, err = generationOutputPath(output)
	if err != nil {
		fmt.Fprintf(stderr, "generate output: %v\n", err)
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
	if experimental || minimize {
		strategy := "best guess"
		if minimize {
			strategy = "minimize"
		}
		fmt.Fprintf(stdout, "experimental %s disabled %d unobserved module options; preserved %d observed hardware modules\n", strategy, optimizedModules, observedModules)
		fmt.Fprintln(stdout, "experimental result: keep the existing working config and review this proposal before installing or booting it")
	}
	return 0
}

func hasHelpSwitch(args []string) bool {
	return slices.Contains(args, "--help") || slices.Contains(args, "-h")
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
	flags.Usage = func() { writeObserveHelp(flags, stderr) }
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
	flags.Usage = func() { writeMigrateHelp(flags, stderr) }
	oldKconfig := flags.String("old-kconfig", "", "old Kconfig artifact")
	newKconfig := flags.String("new-kconfig", "", "new Kconfig artifact")
	oldConfig := flags.String("old-config", "", "old kernel config")
	newConfig := flags.String("new-config", "", "new kernel config")
	root := flags.String("root", "/", "Gentoo installation root")
	procfs := flags.String("procfs", "", "procfs root; default ROOT/proc")
	format := flags.String("format", "text", "output format: text or json")
	colorMode := flags.String("color", "auto", "color output: auto, always, or never")
	verbose := flags.Bool("verbose", false, "include inactive symbol churn")
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
		return runDefaultMigrate(*root, *procfs, *format, mode, *verbose, stdout, stderr)
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
		err = report.MigrationTextWithOptions(
			stdout, changes, report.MigrationTextOptions{
				Style: terminal.StyleForWriter(mode, stdout), Verbose: *verbose,
			},
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
	verbose bool,
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
	oldCatalog, err := kernel.NewCatalog()
	if err != nil {
		fmt.Fprintf(stderr, "migrate: initialize Kconfig catalog: %v\n", err)
		return 1
	}
	for _, tree := range inventory.Trees {
		if !tree.RunningRelease {
			continue
		}
		oldCatalog, err = kernel.LoadKconfigCatalog(ctx, tree.Path)
		if err != nil {
			fmt.Fprintf(stderr, "migrate running Kconfig catalog: %v\n", err)
			return 1
		}
		break
	}
	targetCatalog, err := kernel.LoadKconfigCatalog(ctx, inventory.Target.Path)
	if err != nil {
		fmt.Fprintf(stderr, "migrate target Kconfig catalog: %v\n", err)
		return 1
	}
	changes := kernel.ConfigRelevantChanges(
		kernel.Compare(oldCatalog, targetCatalog, running, validated.Config),
	)
	reasons := make(map[kernel.Symbol][]string)
	evidenceComplete := false
	evidenceStatus := "unavailable"
	inspection, inspectErr := app.InspectSystem(
		ctx, shared.DefaultSystemPaths(root), nil, paths,
		hardware.DefaultSystemPaths(root), time.Time{}, maizegentoo.SnapshotStabilized,
	)
	if inspectErr != nil {
		evidenceStatus = "unavailable: " + inspectErr.Error()
	} else {
		reasons = migrationRecommendationReasons(inspection.Recommendations)
		evidenceComplete = true
		evidenceStatus = fmt.Sprintf(
			"evaluated from %d current kernel recommendations", len(inspection.Recommendations),
		)
	}
	changes = migrationConsumerRelevantChanges(changes, reasons)
	reportContext := report.MigrationContext{
		RunningRelease:   source.RunningRelease,
		RunningConfig:    source.Path,
		TargetRelease:    inventory.Target.Release,
		TargetTree:       inventory.Target.Path,
		ConsumerEvidence: evidenceStatus,
	}
	if format == "json" {
		err = report.MigrationJSONExplained(stdout, reportContext, changes, reasons)
	} else {
		err = report.MigrationTextWithContextOptions(
			stdout, reportContext, changes, report.MigrationTextOptions{
				Style: terminal.StyleForWriter(mode, stdout), Verbose: verbose,
				Reasons: reasons, EvidenceComplete: evidenceComplete,
			},
		)
	}
	if err != nil {
		fmt.Fprintf(stderr, "write migration report: %v\n", err)
		return 1
	}
	return 0
}

func migrationConsumerRelevantChanges(
	changes []kernel.Change,
	reasons map[kernel.Symbol][]string,
) []kernel.Change {
	result := make([]kernel.Change, 0, len(changes))
	for _, change := range changes {
		valueChanged := false
		for _, kind := range change.Kinds {
			if kind == kernel.ChangeValue {
				valueChanged = true
				break
			}
		}
		if valueChanged || len(reasons[change.Symbol]) != 0 {
			result = append(result, change)
		}
	}
	return result
}

func migrationRecommendationReasons(
	recommendations []recommend.Recommendation,
) map[kernel.Symbol][]string {
	result := make(map[kernel.Symbol][]string)
	for _, recommendation := range recommendations {
		values := []string{recommendation.Detail}
		for _, evidence := range recommendation.Evidence {
			if evidence.Detail == recommendation.Detail {
				continue
			}
			detail := evidence.Detail
			if evidence.Source != "" {
				detail += " [" + evidence.Source + "]"
			}
			values = append(values, detail)
		}
		for _, value := range values {
			if value == "" || slices.Contains(result[recommendation.Symbol], value) {
				continue
			}
			result[recommendation.Symbol] = append(result[recommendation.Symbol], value)
		}
	}
	for symbol := range result {
		sort.Strings(result[symbol])
	}
	return result
}

func readKconfig(path string) (kernel.Catalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return kernel.Catalog{}, err
	}
	defer file.Close()
	return kernel.ParseKconfig(path, file)
}

func writeInspectionHelp(flags *flag.FlagSet, command string, writer io.Writer) {
	description := map[string]string{
		"inspect": "Inspect the current kernel, hardware, and effective Gentoo package policy.",
		"check":   "Check whether the current kernel satisfies known hardware and package requirements.",
		"impact":  "Explain which known requirements a proposed kernel configuration would not satisfy.",
	}[command]
	fmt.Fprintf(writer, "Usage:\n  maize %s [options]\n\n%s\n", command, description)
	if command == "impact" {
		fmt.Fprintln(writer, "\nRequired:\n  --config PATH   proposed kernel configuration to evaluate")
	}
	fmt.Fprintln(writer, "\nOptions:")
	printLongFlagDefaults(flags, writer)
	switch command {
	case "inspect":
		fmt.Fprintln(writer, "\nOutput includes the selected config source, hardware inventory, Gentoo snapshot, recommendations, and unresolved policy.")
		fmt.Fprintln(writer, "Exit status: 0 success; 1 inspection/report failure; 2 invalid usage.")
	case "check":
		fmt.Fprintln(writer, "\nExit status: 0 satisfied; 3 known kernel changes required; 4 unresolved package policy; 1 inspection/report failure; 2 invalid usage.")
	case "impact":
		fmt.Fprintln(writer, "\nThis command is read-only and reports only requirements Maize can currently explain.")
		fmt.Fprintln(writer, "Exit status: 0 report produced; 1 inspection/report failure; 2 invalid usage.")
	}
}

func inspectionRoot(args []string) string {
	root := "/"
	for index := 0; index < len(args); index++ {
		if strings.HasPrefix(args[index], "--root=") {
			root = strings.TrimPrefix(args[index], "--root=")
		} else if args[index] == "--root" && index+1 < len(args) {
			root = args[index+1]
			index++
		}
	}
	return root
}

func generationOutputPath(output string) (string, error) {
	info, err := os.Stat(output)
	if err == nil {
		if info.IsDir() {
			return filepath.Join(output, "maize.config"), nil
		}
		return output, nil
	}
	if os.IsNotExist(err) {
		return output, nil
	}
	return "", fmt.Errorf("inspect %q: %w", output, err)
}

func writeDynamicPolicyFailure(writer io.Writer, policies []maizegentoo.DynamicKernelPolicy, verbose bool) {
	type packagePolicies struct {
		name     string
		policies []maizegentoo.DynamicKernelPolicy
	}
	byPackage := make(map[string][]maizegentoo.DynamicKernelPolicy)
	for _, policy := range policies {
		name := policy.Package.CPV()
		byPackage[name] = append(byPackage[name], policy)
	}
	groups := make([]packagePolicies, 0, len(byPackage))
	for name, findings := range byPackage {
		groups = append(groups, packagePolicies{name: name, policies: findings})
	}
	sort.Slice(groups, func(left, right int) bool { return groups[left].name < groups[right].name })
	fmt.Fprintf(writer, "generate: %d dynamic package kernel policies across %d packages require operator review:\n", len(policies), len(groups))
	limit := len(groups)
	if !verbose && limit > 12 {
		limit = 12
	}
	for _, group := range groups[:limit] {
		reasons := make(map[string]bool)
		for _, policy := range group.policies {
			reasons[policy.Reason] = true
		}
		orderedReasons := make([]string, 0, len(reasons))
		for reason := range reasons {
			orderedReasons = append(orderedReasons, reason)
		}
		sort.Strings(orderedReasons)
		fmt.Fprintf(writer, "  %s: %d unresolved finding(s): %s\n", group.name, len(group.policies), strings.Join(orderedReasons, "; "))
		if verbose {
			for _, policy := range group.policies {
				fmt.Fprintf(writer, "    %s", policy.Expression)
				if policy.Provenance.Source != "" {
					fmt.Fprintf(writer, " (%s", policy.Provenance.Source)
					if policy.Provenance.Detail != "" {
						fmt.Fprintf(writer, ": %s", policy.Provenance.Detail)
					}
					fmt.Fprint(writer, ")")
				}
				fmt.Fprintln(writer)
			}
		}
	}
	if limit < len(groups) {
		fmt.Fprintf(writer, "  ... %d more package(s); rerun with --verbose for the complete list\n", len(groups)-limit)
	}
	fmt.Fprintln(writer, "Maize will not guess past shell-dependent package policy; these findings need Gentooling support or operator review.")
}

func writeGenerateHelp(writer io.Writer) {
	fmt.Fprint(writer, `Usage:
  maize generate [--kernel-tree PATH] [--output PATH] [options]

Generate a new configuration from the selected working kernel config, known
hardware, and effective Gentoo package policy. The target kernel resolves
dependencies with olddefconfig in an isolated directory. The source tree and
input configuration are never modified.

Defaults:
  --kernel-tree PATH              target source; default is the newest valid
                                 versioned kernel tree below ROOT/usr/src
  --output PATH                   output file or directory (default
                                 "./maize.config"); a directory receives a
                                 file named maize.config

Experimental strategies (mutually exclusive):
  --experimental-best-guess      prune unobserved modules while retaining
                                 removable-device, filesystem, network, and
                                 audio fallback families
  --experimental-minimize        omit fallback families and attempt the
                                 smallest evidence-supported configuration

Inspection options:
  --root PATH                     Gentoo installation root (default "/")
  --config PATH                   input config; default discovers running,
                                 boot, then source configuration
  --sysfs PATH                    sysfs root (default ROOT/sys)
  --procfs PATH                   procfs root (default ROOT/proc)
  --repository NAME=PATH          repository override; may be repeated
  --snapshot-consistency MODE     locked or stabilized (default "locked")
  --color MODE                    auto, always, or never (default "auto")
  --verbose                       show all supporting and unresolved evidence
  --format text|json              accepted for inspection compatibility;
                                 generation status is textual
  -h, --help                      show this help

Generation refuses unresolved dynamic package policy and verifies that every
required recommendation survives target Kconfig resolution. Experimental
outputs are proposals: disconnected hardware cannot be proven safe.

Exit status: 0 configuration written; 1 inspection, policy, validation, or
write failure; 2 invalid usage.
`)
}

func writeObserveHelp(flags *flag.FlagSet, writer io.Writer) {
	fmt.Fprintln(writer, "Usage:\n  maize observe --output PATH [options]")
	fmt.Fprintln(writer, "\nRecord a versioned JSON inventory of current sysfs hardware and loaded modules. Existing output is replaced atomically only after collection succeeds.")
	fmt.Fprintln(writer, "\nOptions:")
	printLongFlagDefaults(flags, writer)
	fmt.Fprintln(writer, "\nExit status: 0 inventory written; 1 collection/write failure; 2 invalid usage.")
}

func writeMigrateHelp(flags *flag.FlagSet, writer io.Writer) {
	fmt.Fprintln(writer, "Usage:\n  maize migrate [options]\n  maize migrate --old-kconfig PATH --new-kconfig PATH --old-config PATH --new-config PATH [options]")
	fmt.Fprintln(writer, "\nCompare the running kernel with the newest installed kernel source by default. The explicit form compares two supplied Kconfig/config pairs; all four artifact options must be supplied together.")
	fmt.Fprintln(writer, "\nOptions:")
	printLongFlagDefaults(flags, writer)
	fmt.Fprintln(writer, "\nMigration is read-only. Text output groups meaningful consequences; --verbose also includes inactive symbol churn and complete evidence.")
	fmt.Fprintln(writer, "Exit status: 0 report produced; 1 discovery, parsing, validation, or report failure; 2 invalid usage.")
}

func printLongFlagDefaults(flags *flag.FlagSet, writer io.Writer) {
	var defaults bytes.Buffer
	flags.SetOutput(&defaults)
	flags.PrintDefaults()
	flags.SetOutput(writer)
	fmt.Fprint(writer, strings.ReplaceAll(defaults.String(), "  -", "  --"))
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

func takeSwitch(args []string, name string) (bool, []string, error) {
	found := false
	remaining := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == name {
			if found {
				return false, nil, fmt.Errorf("duplicate %s", name)
			}
			found = true
			continue
		}
		if strings.HasPrefix(arg, name+"=") {
			return false, nil, fmt.Errorf("%s does not accept a value", name)
		}
		remaining = append(remaining, arg)
	}
	return found, remaining, nil
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
