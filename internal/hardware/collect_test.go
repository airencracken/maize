package hardware_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/airencracken/maize/internal/hardware"
)

func TestCollectWalksSysfsDevicesDriversAndModules(t *testing.T) {
	t.Parallel()

	paths := sysfsFixture(t)
	observed := time.Unix(1_700_000_000, 0).UTC()
	inventory, err := hardware.Collect(
		context.Background(), paths, hardware.CollectOptions{ObservedAt: observed},
	)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.Schema != 1 || len(inventory.Devices) != 2 {
		t.Fatalf("inventory = %#v", inventory)
	}
	if !reflect.DeepEqual(inventory.LoadedModules, []string{"iwlwifi", "zfs"}) {
		t.Fatalf("loaded modules = %v", inventory.LoadedModules)
	}
	var pci hardware.Device
	for _, device := range inventory.Devices {
		if device.Bus == hardware.BusPCI {
			pci = device
			break
		}
	}
	if pci.Bus != hardware.BusPCI ||
		pci.ID.Address != "0000:00:14.3" ||
		pci.ID.Vendor != "8086" ||
		pci.ID.Product != "51f1" ||
		pci.Driver != "iwlwifi" ||
		!reflect.DeepEqual(pci.Modules, []string{"iwlwifi"}) ||
		pci.Provenance[0].ObservedAt != observed {
		t.Fatalf("PCI device = %#v", pci)
	}
}

func TestCollectIsDeterministicAndReturnsOwnedInventories(t *testing.T) {
	t.Parallel()

	paths := sysfsFixture(t)
	options := hardware.CollectOptions{ObservedAt: time.Unix(123, 0).UTC()}
	first, err := hardware.Collect(context.Background(), paths, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hardware.Collect(context.Background(), paths, options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("inventories differ:\nfirst  %#v\nsecond %#v", first, second)
	}
	first.Devices[0].Modules = append(first.Devices[0].Modules, "mutated")
	if reflect.DeepEqual(first, second) {
		t.Fatal("inventory results alias")
	}
}

func TestCollectRejectsSysfsEscapesAtomically(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sys := filepath.Join(root, "sys")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(filepath.Join(sys, "bus", "pci", "devices"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sys, "bus", "pci", "devices", "escape")); err != nil {
		t.Fatal(err)
	}
	inventory, err := hardware.Collect(
		context.Background(), hardware.SystemPaths{Sys: sys},
		hardware.CollectOptions{ObservedAt: time.Unix(1, 0)},
	)
	if err == nil {
		t.Fatal("sysfs escape accepted")
	}
	if !reflect.DeepEqual(inventory, hardware.Inventory{}) {
		t.Fatalf("escape returned partial inventory: %#v", inventory)
	}
}

func TestCollectHonorsCancellationAndRejectsInvalidRoot(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inventory, err := hardware.Collect(ctx, sysfsFixture(t), hardware.CollectOptions{})
	if !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(inventory, hardware.Inventory{}) {
		t.Fatalf("cancellation returned %#v, %v", inventory, err)
	}

	file := filepath.Join(t.TempDir(), "sys")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	inventory, err = hardware.Collect(
		context.Background(), hardware.SystemPaths{Sys: file}, hardware.CollectOptions{},
	)
	if err == nil || !reflect.DeepEqual(inventory, hardware.Inventory{}) {
		t.Fatalf("invalid root returned %#v, %v", inventory, err)
	}
}

func sysfsFixture(t *testing.T) hardware.SystemPaths {
	t.Helper()

	root := t.TempDir()
	sys := filepath.Join(root, "sys")
	proc := filepath.Join(root, "proc")
	device := filepath.Join(sys, "devices", "pci0000:00", "0000:00:14.3")
	driver := filepath.Join(sys, "bus", "pci", "drivers", "iwlwifi")
	module := filepath.Join(sys, "module", "iwlwifi")
	for _, directory := range []string{
		device, driver, module, proc,
		filepath.Join(sys, "bus", "pci", "devices"),
		filepath.Join(sys, "devices", "system", "cpu", "cpu0"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(proc, "modules"), []byte("zfs 1 0 - Live 0x0\niwlwifi 2 1 - Live 0x0\nzfs 1 0 - Live 0x0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"vendor": "0x8086\n", "device": "0x51f1\n",
		"class": "0x028000\n", "modalias": "pci:v00008086d000051F1\n",
	} {
		if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(device, filepath.Join(sys, "bus", "pci", "devices", "0000:00:14.3")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(driver, filepath.Join(device, "driver")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(module, filepath.Join(driver, "module")); err != nil {
		t.Fatal(err)
	}
	return hardware.SystemPaths{Sys: sys, Proc: proc}
}
