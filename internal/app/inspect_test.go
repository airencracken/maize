package app_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	shared "github.com/airencracken/gentooling"
	"github.com/airencracken/maize/internal/app"
	"github.com/airencracken/maize/internal/recommend"
)

func TestInspectRunsGentooPackageToKernelPipeline(t *testing.T) {
	t.Parallel()

	paths := inspectionFixture(t)
	inspection, err := app.Inspect(
		context.Background(), paths, nil, "old.config",
		strings.NewReader(
			"CONFIG_CGROUPS=y\nCONFIG_NAMESPACES=y\nCONFIG_SECCOMP=y\n"+
				"# CONFIG_SECCOMP_FILTER is not set\n",
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Schema != app.InspectSchema || inspection.InstalledCount != 1 ||
		len(inspection.WorldSelections) != 1 ||
		len(inspection.SystemSelections) != 1 ||
		len(inspection.Recommendations) != 4 {
		t.Fatalf("inspection = %#v", inspection)
	}
	var filterAction recommend.Action
	for _, item := range inspection.Recommendations {
		if item.Symbol.String() == "CONFIG_SECCOMP_FILTER" {
			filterAction = item.Action
			if len(item.Evidence) != 1 ||
				item.Evidence[0].Source != "app-containers/docker-28.3.2[seccomp]" {
				t.Fatalf("filter evidence = %#v", item.Evidence)
			}
		}
	}
	if filterAction != recommend.ActionEnable {
		t.Fatalf("seccomp filter action = %q", filterAction)
	}
}

func TestInspectPreservesConfigAndSnapshotErrorsAtomically(t *testing.T) {
	t.Parallel()

	inspection, err := app.Inspect(
		context.Background(), shared.SystemPaths{}, nil, "bad.config",
		strings.NewReader("not a config\n"),
	)
	if err == nil || inspection.Schema != "" {
		t.Fatalf("invalid config returned %#v, %v", inspection, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inspection, err = app.Inspect(
		ctx, inspectionFixture(t), nil, "config",
		strings.NewReader("CONFIG_SECCOMP=y\n"),
	)
	if err == nil || inspection.Schema != "" {
		t.Fatalf("cancellation returned %#v, %v", inspection, err)
	}
}

func inspectionFixture(t *testing.T) shared.SystemPaths {
	t.Helper()

	root := t.TempDir()
	repository := filepath.Join(root, "var", "db", "repos", "gentoo")
	profile := filepath.Join(repository, "profiles", "default", "linux", "amd64")
	write(t, filepath.Join(repository, "profiles", "base", "packages"), "*sys-apps/baselayout\n")
	write(t, filepath.Join(profile, "parent"), "../../../base\n")
	write(t, filepath.Join(profile, "make.defaults"), "ARCH=\"amd64\"\n")
	active := filepath.Join(root, "etc", "portage", "make.profile")
	if err := os.MkdirAll(filepath.Dir(active), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(profile, active); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "usr", "share", "portage", "config", "make.globals"), "USE=\"\"\n")
	write(t, filepath.Join(root, "etc", "portage", "make.conf"), "USE=\"\"\n")
	write(t, filepath.Join(root, "var", "lib", "portage", "world"), "app-containers/docker\n")
	vdb := filepath.Join(root, "var", "db", "pkg", "app-containers", "docker-28.3.2")
	for name, value := range map[string]string{
		"CONTENTS": "", "EAPI": "8", "SLOT": "0", "repository": "gentoo",
		"IUSE": "seccomp apparmor", "USE": "seccomp",
	} {
		write(t, filepath.Join(vdb, name), value)
	}
	paths := shared.DefaultSystemPaths(root)
	paths.Repositories = []shared.RepositoryPath{{Name: "gentoo", Path: repository}}
	return paths
}

func write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
