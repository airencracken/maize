package gentooling_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
)

func TestReadEffectiveConfigPreservesLayersExpansionAndProvenance(t *testing.T) {
	t.Parallel()

	paths := effectiveConfigFixture(t)
	evidence, err := maizegentoo.ReadEffectiveConfig(
		context.Background(),
		paths,
		[]string{"USE=command", "PYTHON_TARGETS=python3_15", "SECRET=ignored"},
	)
	if err != nil {
		t.Fatalf("read effective config: %v", err)
	}
	if evidence.Profile == nil || len(evidence.Profile.Layers) != 2 {
		t.Fatalf("profile graph missing: %#v", evidence.Profile)
	}
	if evidence.Variables["CFLAGS"] != "-O1 -O2" || evidence.Variables["SECRET"] != "" {
		t.Fatalf("effective variables = %#v", evidence.Variables)
	}
	if len(evidence.UseChanges) != 8 {
		t.Fatalf("USE changes = %#v", evidence.UseChanges)
	}
	command := evidence.UseChanges[len(evidence.UseChanges)-1]
	if command.Name != "python_targets_python3_15" ||
		command.Layer != maizegentoo.ConfigCommand ||
		command.Provenance.Source != "environment" {
		t.Fatalf("command USE provenance = %#v", command)
	}
	if len(evidence.UserPackagePolicy) != 2 ||
		evidence.UserPackagePolicy[0].Value != "app-misc/example" ||
		evidence.UserPackagePolicy[0].Provenance.Kind != domain.SourceConfig ||
		filepath.Base(evidence.UserPackagePolicy[0].Provenance.Source) != "10-base" {
		t.Fatalf("package.use provenance = %#v", evidence.UserPackagePolicy)
	}
}

func TestReadEffectiveConfigDoesNotImportProcessEnvironment(t *testing.T) {
	paths := effectiveConfigFixture(t)
	t.Setenv("USE", "must-not-enter")

	evidence, err := maizegentoo.ReadEffectiveConfig(context.Background(), paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range evidence.UseChanges {
		if change.Name == "must-not-enter" {
			t.Fatal("process environment entered effective configuration")
		}
	}
}

func TestReadEffectiveConfigReturnsOwnedEvidence(t *testing.T) {
	t.Parallel()

	paths := effectiveConfigFixture(t)
	evidence, err := maizegentoo.ReadEffectiveConfig(context.Background(), paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Variables["USE"] = "mutated"
	evidence.UseExpand[0] = "MUTATED"
	evidence.UserPackagePolicy[0].Flags[0] = "mutated"
	evidence.Profile.Layers[0].MakeDefaults["ARCH"] = "mutated"

	again, err := maizegentoo.ReadEffectiveConfig(context.Background(), paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.Variables["USE"] == "mutated" ||
		again.UseExpand[0] == "MUTATED" ||
		again.UserPackagePolicy[0].Flags[0] == "mutated" ||
		again.Profile.Layers[0].MakeDefaults["ARCH"] == "mutated" {
		t.Fatalf("effective configuration aliases prior result: %#v", again)
	}
}

func TestReadEffectiveConfigPreservesErrorsAtomically(t *testing.T) {
	t.Parallel()

	paths := effectiveConfigFixture(t)
	if err := os.Remove(paths.MakeGlobals); err != nil {
		t.Fatal(err)
	}
	evidence, err := maizegentoo.ReadEffectiveConfig(context.Background(), paths, nil)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error %v, want os.ErrNotExist", err)
	}
	if evidence.Variables != nil || evidence.Profile != nil || evidence.UseChanges != nil {
		t.Fatalf("error returned partial evidence: %#v", evidence)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = maizegentoo.ReadEffectiveConfig(ctx, paths, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
}

func effectiveConfigFixture(t *testing.T) shared.SystemPaths {
	t.Helper()

	paths, _, leaf := profileFixture(t)
	root := filepath.Dir(filepath.Dir(filepath.Dir(paths.ActiveProfile)))
	paths.ConfigRoot = filepath.Join(root, "etc", "portage")
	paths.MakeGlobals = filepath.Join(root, "usr", "share", "portage", "config", "make.globals")
	writeFile(t, filepath.Dir(paths.MakeGlobals), "make.globals",
		"FEATURES=\"sandbox\"\nUSE_EXPAND=\"PYTHON_TARGETS\"\n")
	writeFile(t, leaf, "make.defaults",
		"ARCH=\"amd64\"\nUSE=\"profile\"\nPYTHON_TARGETS=\"python3_13\"\nCFLAGS=\"-O1\"\n")
	writeFile(t, paths.ConfigRoot, "make.conf",
		"USE=\"user -profile\"\nPYTHON_TARGETS=\"python3_14 -python3_13\"\nCFLAGS=\"${CFLAGS} -O2\"\n")
	packageUse := filepath.Join(paths.ConfigRoot, "package.use")
	writeFile(t, packageUse, "10-base", "app-misc/example feature\n")
	writeFile(t, packageUse, "20-local", "app-misc/example -feature local\n")
	return paths
}
