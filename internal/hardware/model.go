// Package hardware defines host hardware evidence without prescribing how it
// is collected. Linux sysfs collectors can be added independently.
package hardware

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/airencracken/maize/internal/domain"
)

type Bus string

const (
	BusPCI         Bus = "pci"
	BusUSB         Bus = "usb"
	BusPlatform    Bus = "platform"
	BusSCSI        Bus = "scsi"
	BusNVMe        Bus = "nvme"
	BusInput       Bus = "input"
	BusI2C         Bus = "i2c"
	BusSPI         Bus = "spi"
	BusThunderbolt Bus = "thunderbolt"
	BusCPU         Bus = "cpu"
	BusFirmware    Bus = "firmware"
)

type Presence string

const (
	Present    Presence = "present"
	Historical Presence = "historical"
	Declared   Presence = "declared"
)

type Identifier struct {
	Vendor  string
	Product string
	Class   string
	Address string
}

type Device struct {
	Bus        Bus
	ID         Identifier
	Name       string
	Driver     string
	Modules    []string
	Firmware   []string
	Presence   Presence
	Provenance []domain.Provenance
}

func (d Device) Validate() error {
	if !slices.Contains([]Bus{
		BusPCI, BusUSB, BusPlatform, BusSCSI, BusNVMe, BusInput,
		BusI2C, BusSPI, BusThunderbolt, BusCPU, BusFirmware,
	}, d.Bus) {
		return fmt.Errorf("invalid hardware bus %q", d.Bus)
	}
	if !slices.Contains([]Presence{Present, Historical, Declared}, d.Presence) {
		return fmt.Errorf("invalid hardware presence %q", d.Presence)
	}
	if strings.TrimSpace(d.ID.Address) == "" &&
		strings.TrimSpace(d.ID.Product) == "" &&
		strings.TrimSpace(d.Name) == "" {
		return errors.New("hardware device requires an address, product, or name")
	}
	for index, provenance := range d.Provenance {
		if err := provenance.Validate(); err != nil {
			return fmt.Errorf("provenance %d: %w", index, err)
		}
	}
	return nil
}

type Inventory struct {
	Schema  uint
	Devices []Device
}

func (i Inventory) Validate() error {
	if i.Schema != 1 {
		return fmt.Errorf("unsupported hardware inventory schema %d", i.Schema)
	}
	seen := make(map[string]bool)
	for index, device := range i.Devices {
		if err := device.Validate(); err != nil {
			return fmt.Errorf("device %d: %w", index, err)
		}
		key := string(device.Bus) + "\x00" + device.ID.Address + "\x00" +
			device.ID.Vendor + "\x00" + device.ID.Product
		if seen[key] {
			return fmt.Errorf("device %d duplicates hardware identity", index)
		}
		seen[key] = true
	}
	return nil
}
