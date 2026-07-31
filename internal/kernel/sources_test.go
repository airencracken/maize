package kernel

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverSourceTreesSelectsLatestKernelVersionAndMarksRunning(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourceTreeFixture(t, root, "linux-6.9.12-gentoo", "")
	sourceTreeFixture(t, root, "linux-6.10.2-gentoo", "")
	latest := sourceTreeFixture(t, root, "linux-6.10.10-gentoo", "6.10.10-gentoo-r1")
	sourceTreeFixture(t, root, "linux-6.11.0-rc2", "")
	if err := os.Symlink(latest, filepath.Join(root, "usr", "src", "linux")); err != nil {
		t.Fatal(err)
	}
	inventory, err := DiscoverSourceTrees(root, "6.10.2-gentoo")
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Trees) != 4 ||
		inventory.Target.Release != "6.11.0-rc2" ||
		!inventory.Trees[1].RunningRelease ||
		inventory.Trees[1].Release != "6.10.2-gentoo" {
		t.Fatalf("inventory = %#v", inventory)
	}
}

func TestKernelReleaseOrderingHandlesNumericRevisionsAndPrereleases(t *testing.T) {
	t.Parallel()

	ordered := []string{
		"6.9.12-gentoo", "6.10.0-rc2", "6.10.0", "6.10.0-gentoo",
		"6.10.0-gentoo-r1", "6.10.0-gentoo-r10", "6.10.2",
	}
	for index := 1; index < len(ordered); index++ {
		if compareKernelRelease(ordered[index-1], ordered[index]) >= 0 {
			t.Errorf("%q did not sort before %q", ordered[index-1], ordered[index])
		}
		if compareKernelRelease(ordered[index], ordered[index-1]) <= 0 {
			t.Errorf("%q did not sort after %q", ordered[index], ordered[index-1])
		}
	}
}

func TestDiscoverSourceTreesRejectsMissingInventoryAtomically(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "usr", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourceTreeFixture(t, root, "not-linux-7.0", "")
	inventory, err := DiscoverSourceTrees(root, "")
	if err == nil || !reflect.DeepEqual(inventory, SourceInventory{}) {
		t.Fatalf("inventory %#v, error %v", inventory, err)
	}
}

func TestDiscoverSourceTreesIgnoresMalformedAndIncompleteCandidates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	valid := sourceTreeFixture(t, root, "linux-6.12.1", "")
	if err := os.MkdirAll(filepath.Join(root, "usr", "src", "linux-999.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "usr", "src", "linux-escape"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	for _, file := range []string{"Kconfig", "Makefile"} {
		if err := os.WriteFile(filepath.Join(outside, file), []byte("hostile\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "usr", "src", "linux-999.1")); err != nil {
		t.Fatal(err)
	}
	inventory, err := DiscoverSourceTrees(root, "")
	if err != nil || len(inventory.Trees) != 1 || inventory.Target.Path != valid {
		t.Fatalf("inventory %#v, error %v", inventory, err)
	}
}

func sourceTreeFixture(t *testing.T, root, name, release string) string {
	t.Helper()
	path := filepath.Join(root, "usr", "src", name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"Kconfig", "Makefile"} {
		if err := os.WriteFile(filepath.Join(path, file), []byte("fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if release != "" {
		releasePath := filepath.Join(path, "include", "config", "kernel.release")
		if err := os.MkdirAll(filepath.Dir(releasePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(releasePath, []byte(release+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}
