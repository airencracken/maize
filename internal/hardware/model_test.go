package hardware_test

import (
	"testing"
	"time"

	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/hardware"
)

func TestInventoryValidatesProvenance(t *testing.T) {
	t.Parallel()

	inventory := hardware.Inventory{Schema: 1, Devices: []hardware.Device{{
		Bus: hardware.BusPCI,
		ID: hardware.Identifier{
			Vendor: "8086", Product: "7a60", Class: "028000", Address: "0000:00:14.3",
		},
		Driver:   "iwlwifi",
		Modules:  []string{"iwlwifi"},
		Presence: hardware.Present,
		Provenance: []domain.Provenance{{
			Kind: domain.SourceDevice, Source: "/sys/bus/pci/devices/0000:00:14.3",
			Detail: "device and bound driver observed in sysfs", ObservedAt: time.Unix(1_700_000_000, 0),
		}},
	}}}
	if err := inventory.Validate(); err != nil {
		t.Fatalf("valid inventory rejected: %v", err)
	}
}

func TestInventoryRejectsDuplicateAndHostileDevices(t *testing.T) {
	t.Parallel()

	device := hardware.Device{
		Bus: hardware.BusPCI, ID: hardware.Identifier{Address: "0000:00:14.3"},
		Presence: hardware.Present,
	}
	if err := (hardware.Inventory{Schema: 1, Devices: []hardware.Device{device, device}}).Validate(); err == nil {
		t.Fatal("duplicate identity accepted")
	}
	device.Bus = hardware.Bus("../../hostile")
	if err := device.Validate(); err == nil {
		t.Fatal("invalid bus accepted")
	}
}
