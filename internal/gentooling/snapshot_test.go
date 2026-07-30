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
)

func TestReadSystemSnapshotReturnsConsistentTranslatedEvidence(t *testing.T) {
	t.Parallel()

	paths := systemSnapshotFixture(t)
	snapshot, err := maizegentoo.ReadSystemSnapshot(
		context.Background(), paths, []string{"USE=command"}, 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Installed.Packages) != 1 ||
		snapshot.Installed.Packages[0].ID.CPV() != "app-misc/example-1" ||
		snapshot.Installed.Packages[0].Contents != "" {
		t.Fatalf("installed evidence = %#v", snapshot.Installed)
	}
	if snapshot.Config.Profile == nil ||
		snapshot.Config.Variables["USE"] != "command" {
		t.Fatalf("config evidence = %#v", snapshot.Config)
	}
	if len(snapshot.Selections.World) != 2 ||
		snapshot.Selections.World[0].Value != "app-misc/example" ||
		snapshot.Selections.World[0].Kind != maizegentoo.SelectionPackage ||
		snapshot.Selections.World[0].Provenance.Kind != domain.SourceConfig ||
		snapshot.Selections.World[1].Value != "@preserved-rebuild" ||
		snapshot.Selections.World[1].Kind != maizegentoo.SelectionSet {
		t.Fatalf("world selections = %#v", snapshot.Selections.World)
	}
	if len(snapshot.Selections.System) != 1 ||
		snapshot.Selections.System[0].Value != "sys-apps/baselayout" ||
		snapshot.Selections.System[0].Provenance.Kind != domain.SourceProfile ||
		snapshot.Selections.System[0].Provenance.Source == "" {
		t.Fatalf("system selections = %#v", snapshot.Selections.System)
	}
}

func TestReadSystemSnapshotReturnsOwnedDeterministicEvidence(t *testing.T) {
	t.Parallel()

	paths := systemSnapshotFixture(t)
	first, err := maizegentoo.ReadSystemSnapshot(context.Background(), paths, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := maizegentoo.ReadSystemSnapshot(context.Background(), paths, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("snapshots differ:\nfirst  %#v\nsecond %#v", first, second)
	}
	first.Selections.World[0].Value = "mutated"
	first.Config.Variables["ARCH"] = "mutated"
	if second.Selections.World[0].Value == "mutated" ||
		second.Config.Variables["ARCH"] == "mutated" {
		t.Fatal("snapshot evidence aliases another read")
	}
}

func TestReadSystemSnapshotPreservesErrorsAtomically(t *testing.T) {
	t.Parallel()

	paths := systemSnapshotFixture(t)
	if err := os.WriteFile(paths.World, []byte("invalid world entry\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := maizegentoo.ReadSystemSnapshot(context.Background(), paths, nil, 2)
	if !errors.Is(err, shared.ErrInvalidData) {
		t.Fatalf("error %v, want ErrInvalidData", err)
	}
	if !reflect.DeepEqual(snapshot, maizegentoo.SystemSnapshotEvidence{}) {
		t.Fatalf("error returned partial snapshot: %#v", snapshot)
	}

	snapshot, err = maizegentoo.ReadSystemSnapshot(context.Background(), paths, nil, 1)
	if !errors.Is(err, shared.ErrInvalidData) {
		t.Fatalf("attempt error %v, want ErrInvalidData", err)
	}
	if !reflect.DeepEqual(snapshot, maizegentoo.SystemSnapshotEvidence{}) {
		t.Fatalf("invalid attempts returned partial snapshot: %#v", snapshot)
	}
}

func TestReadSystemSnapshotHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snapshot, err := maizegentoo.ReadSystemSnapshot(ctx, systemSnapshotFixture(t), nil, 2)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(snapshot, maizegentoo.SystemSnapshotEvidence{}) {
		t.Fatalf("cancellation returned partial snapshot: %#v", snapshot)
	}
}

func systemSnapshotFixture(t *testing.T) shared.SystemPaths {
	t.Helper()

	paths := effectiveConfigFixture(t)
	root := filepath.Dir(filepath.Dir(filepath.Dir(paths.ActiveProfile)))
	paths.VDB = filepath.Join(root, "var", "db", "pkg")
	paths.World = filepath.Join(root, "var", "lib", "portage", "world")
	writeInstalled(t, paths.VDB, "app-misc/example-1", map[string]string{
		"IUSE": "+feature local",
		"USE":  "feature",
	})
	writeFile(t, filepath.Dir(paths.World), filepath.Base(paths.World),
		"app-misc/example\n@preserved-rebuild\n")
	return paths
}
