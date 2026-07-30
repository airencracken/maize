package gentooling_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/domain"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
	"github.com/airencracken/maize/internal/resolve"
)

func TestReadInstalledUsesStrictEvidenceAndExcludesContents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeInstalled(t, root, "app-containers/docker-28.3.2", map[string]string{
		"USE":      "seccomp",
		"IUSE":     "seccomp apparmor",
		"CONTENTS": "obj /usr/bin/docker digest 1",
	})

	inventory, err := maizegentoo.ReadInstalled(
		context.Background(),
		shared.SystemPaths{VDB: root},
	)
	if err != nil {
		t.Fatalf("read installed: %v", err)
	}
	if len(inventory.Packages) != 1 {
		t.Fatalf("got %d packages, want 1", len(inventory.Packages))
	}
	if inventory.Packages[0].Contents != "" {
		t.Fatalf("CONTENTS leaked into resolver evidence: %q", inventory.Packages[0].Contents)
	}
}

func TestReadInstalledRejectsIncompleteEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeInstalled(t, root, "app-containers/docker-28.3.2", map[string]string{
		"CONTENTS": absent,
	})

	inventory, err := maizegentoo.ReadInstalled(
		context.Background(),
		shared.SystemPaths{VDB: root},
	)
	if !errors.Is(err, shared.ErrIncompleteEvidence) {
		t.Fatalf("error %v, want ErrIncompleteEvidence", err)
	}
	if len(inventory.Issues) != 1 || inventory.Issues[0].Code != shared.IssueInterruptedRecord {
		t.Fatalf("unexpected issues: %#v", inventory.Issues)
	}
}

func TestInstalledPackageEvidenceResolvesEndToEnd(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeInstalled(t, root, "app-containers/docker-28.3.2", map[string]string{
		"USE":  "seccomp",
		"IUSE": "seccomp apparmor",
	})

	inventory, err := maizegentoo.ReadInstalled(
		context.Background(),
		shared.SystemPaths{VDB: root},
	)
	if err != nil {
		t.Fatalf("read installed: %v", err)
	}
	requirements, err := maizegentoo.Requirements(inventory, []maizegentoo.Rule{{
		Package:     "app-containers/docker",
		UseFlag:     "seccomp",
		Capability:  "security.seccomp-filter",
		Disposition: domain.Required,
		Confidence:  domain.Certain,
		Detail:      "Docker was built with seccomp support",
	}})
	if err != nil {
		t.Fatalf("translate requirements: %v", err)
	}
	decisions, err := resolve.Requirements(requirements)
	if err != nil {
		t.Fatalf("resolve requirements: %v", err)
	}
	if len(decisions) != 1 ||
		decisions[0].Capability != "security.seccomp-filter" ||
		decisions[0].Disposition != domain.Required {
		t.Fatalf("unexpected decisions: %#v", decisions)
	}
}

func TestReadInstalledHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := maizegentoo.ReadInstalled(ctx, shared.SystemPaths{VDB: t.TempDir()})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
}

func TestRequirementsUsesInstalledPackageAndRecordedUseEvidence(t *testing.T) {
	t.Parallel()

	inventory := shared.InstalledInventory{Packages: []shared.InstalledPackage{
		{
			ID:         shared.PackageID{Category: "app-containers", Name: "docker", Version: "28.3.2"},
			EnabledUse: []string{"seccomp"},
		},
	}}
	rules := []maizegentoo.Rule{
		{
			Package: "app-containers/docker", UseFlag: "seccomp",
			Capability: "security.seccomp-filter", Disposition: domain.Required,
			Confidence: domain.Certain, Detail: "Docker was built with seccomp support",
		},
		{
			Package: "app-containers/docker", UseFlag: "apparmor",
			Capability: "security.apparmor", Disposition: domain.Required,
			Confidence: domain.Certain, Detail: "Docker was built with AppArmor support",
		},
		{
			Package:    "app-containers/docker",
			Capability: "containers", Disposition: domain.Recommended,
			Confidence: domain.High, Detail: "Docker is installed",
		},
	}

	got, err := maizegentoo.Requirements(inventory, rules)
	if err != nil {
		t.Fatalf("requirements: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d requirements, want 2: %#v", len(got), got)
	}
	if got[0].Capability != "containers" || got[0].Evidence.Kind != domain.SourcePackage {
		t.Fatalf("unexpected package evidence: %#v", got[0])
	}
	if got[1].Capability != "security.seccomp-filter" ||
		got[1].Evidence.Kind != domain.SourceUseFlag ||
		got[1].Evidence.Source != "app-containers/docker-28.3.2[seccomp]" {
		t.Fatalf("unexpected USE evidence: %#v", got[1])
	}
}

func TestRequirementsIsOrderIndependent(t *testing.T) {
	t.Parallel()

	firstPackage := shared.InstalledPackage{
		ID:         shared.PackageID{Category: "sys-fs", Name: "cryptsetup", Version: "2.8.1"},
		EnabledUse: []string{"udev"},
	}
	secondPackage := shared.InstalledPackage{
		ID: shared.PackageID{Category: "app-containers", Name: "docker", Version: "28.3.2"},
	}
	firstRule := maizegentoo.Rule{
		Package: "sys-fs/cryptsetup", Capability: "storage.dm-crypt",
		Disposition: domain.Recommended, Confidence: domain.High,
		Detail: "cryptsetup is installed",
	}
	secondRule := maizegentoo.Rule{
		Package: "app-containers/docker", Capability: "containers",
		Disposition: domain.Recommended, Confidence: domain.High,
		Detail: "Docker is installed",
	}

	left, err := maizegentoo.Requirements(
		shared.InstalledInventory{Packages: []shared.InstalledPackage{firstPackage, secondPackage}},
		[]maizegentoo.Rule{firstRule, secondRule},
	)
	if err != nil {
		t.Fatal(err)
	}
	right, err := maizegentoo.Requirements(
		shared.InstalledInventory{Packages: []shared.InstalledPackage{secondPackage, firstPackage}},
		[]maizegentoo.Rule{secondRule, firstRule},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("order changed requirements:\nleft  %#v\nright %#v", left, right)
	}
}

func TestRequirementsRejectsInvalidRulesAtomically(t *testing.T) {
	t.Parallel()

	inventory := shared.InstalledInventory{Packages: []shared.InstalledPackage{{
		ID: shared.PackageID{Category: "app-containers", Name: "docker", Version: "28.3.2"},
	}}}
	rules := []maizegentoo.Rule{
		{
			Package: "app-containers/docker", Capability: "containers",
			Disposition: domain.Required, Confidence: domain.Certain,
			Detail: "valid evidence",
		},
		{
			Package: "../escape", Capability: "hostile",
			Disposition: domain.Required, Confidence: domain.Certain,
			Detail: "invalid evidence",
		},
	}

	got, err := maizegentoo.Requirements(inventory, rules)
	if err == nil {
		t.Fatal("invalid rule accepted")
	}
	if got != nil {
		t.Fatalf("invalid rule returned partial requirements: %#v", got)
	}
}

func TestRuleValidationRejectsAdversarialInputs(t *testing.T) {
	t.Parallel()

	tests := []maizegentoo.Rule{
		{},
		{
			Package: "../escape", Capability: "valid", Disposition: domain.Required,
			Confidence: domain.Certain, Detail: "test",
		},
		{
			Package: "app-misc/example", UseFlag: "flag\ninjected",
			Capability: "valid", Disposition: domain.Required,
			Confidence: domain.Certain, Detail: "test",
		},
		{
			Package: "app-misc/example", Capability: "CONFIG_RAW_SYMBOL",
			Disposition: domain.Required, Confidence: domain.Certain, Detail: "test",
		},
	}
	for index, rule := range tests {
		if err := rule.Validate(); err == nil {
			t.Errorf("rule %d accepted: %#v", index, rule)
		}
	}
}

func FuzzRuleValidationNeverPanics(f *testing.F) {
	f.Add("app-containers/docker", "seccomp", "security.seccomp-filter")
	f.Add("../escape", "\x00", "CONFIG_HOSTILE")

	f.Fuzz(func(t *testing.T, pkg, use, capability string) {
		rule := maizegentoo.Rule{
			Package: pkg, UseFlag: use, Capability: capability,
			Disposition: domain.Required, Confidence: domain.Low, Detail: "fuzz input",
		}
		_ = rule.Validate()
	})
}

const absent = "\x00absent\x00"

func writeInstalled(t *testing.T, root, cpv string, overrides map[string]string) {
	t.Helper()

	id, err := shared.ParsePackageID(cpv)
	if err != nil {
		t.Fatalf("parse fixture identity: %v", err)
	}
	directory := filepath.Join(root, id.Category, id.Name+"-"+id.Version)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	files := map[string]string{
		"CONTENTS":   "",
		"EAPI":       "8",
		"SLOT":       "0",
		"repository": "gentoo",
	}
	for name, value := range overrides {
		files[name] = value
	}
	for name, value := range files {
		if value == absent {
			continue
		}
		if err := os.WriteFile(filepath.Join(directory, name), []byte(value), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
}
