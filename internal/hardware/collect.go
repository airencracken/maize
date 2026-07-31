package hardware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/airencracken/maize/internal/domain"
)

const (
	maxDevices       = 65536
	maxAttributeSize = 1024 * 1024
)

type SystemPaths struct {
	Sys  string
	Proc string
}

func DefaultSystemPaths(root string) SystemPaths {
	clean := filepath.Clean(root)
	return SystemPaths{
		Sys: filepath.Join(clean, "sys"), Proc: filepath.Join(clean, "proc"),
	}
}

type CollectOptions struct {
	ObservedAt time.Time
}

// Collect walks supported sysfs device buses without executing system tools.
// Device links may resolve within Sys but escapes, malformed attributes, and
// oversized inventories are rejected atomically.
func Collect(
	ctx context.Context,
	paths SystemPaths,
	options CollectOptions,
) (Inventory, error) {
	if err := ctx.Err(); err != nil {
		return Inventory{}, err
	}
	sysRoot, err := validateSysRoot(paths.Sys)
	if err != nil {
		return Inventory{}, err
	}
	observedAt := options.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	sources := []struct {
		bus  Bus
		path string
	}{
		{BusPCI, filepath.Join(sysRoot, "bus", "pci", "devices")},
		{BusUSB, filepath.Join(sysRoot, "bus", "usb", "devices")},
		{BusPlatform, filepath.Join(sysRoot, "bus", "platform", "devices")},
		{BusSCSI, filepath.Join(sysRoot, "bus", "scsi", "devices")},
		{BusNVMe, filepath.Join(sysRoot, "bus", "nvme", "devices")},
		{BusInput, filepath.Join(sysRoot, "bus", "input", "devices")},
		{BusI2C, filepath.Join(sysRoot, "bus", "i2c", "devices")},
		{BusSPI, filepath.Join(sysRoot, "bus", "spi", "devices")},
		{BusThunderbolt, filepath.Join(sysRoot, "bus", "thunderbolt", "devices")},
		{BusCPU, filepath.Join(sysRoot, "devices", "system", "cpu")},
	}
	var devices []Device
	for _, source := range sources {
		collected, collectErr := collectDirectory(ctx, sysRoot, source.path, source.bus, observedAt)
		if collectErr != nil {
			return Inventory{}, collectErr
		}
		devices = append(devices, collected...)
		if len(devices) > maxDevices {
			return Inventory{}, fmt.Errorf("hardware inventory exceeds %d devices", maxDevices)
		}
	}
	sort.SliceStable(devices, func(left, right int) bool {
		if devices[left].Bus != devices[right].Bus {
			return devices[left].Bus < devices[right].Bus
		}
		return devices[left].ID.Address < devices[right].ID.Address
	})
	var loadedModules []string
	if paths.Proc != "" {
		loadedModules, err = readLoadedModules(filepath.Join(paths.Proc, "modules"))
		if err != nil {
			return Inventory{}, err
		}
	}
	inventory := Inventory{Schema: 1, Devices: devices, LoadedModules: loadedModules}
	if err := inventory.Validate(); err != nil {
		return Inventory{}, err
	}
	return inventory, nil
}

func readLoadedModules(path string) ([]string, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open loaded modules: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("read loaded modules: %w", err)
	}
	if len(data) > 8*1024*1024 {
		return nil, fmt.Errorf("loaded modules exceeds size limit")
	}
	seen := make(map[string]bool)
	var modules []string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if strings.Contains(name, "/") || seen[name] {
			continue
		}
		seen[name] = true
		modules = append(modules, name)
	}
	sort.Strings(modules)
	return modules, nil
}

func validateSysRoot(path string) (string, error) {
	if path == "" {
		return "", errors.New("sysfs root is required")
	}
	clean := filepath.Clean(path)
	info, err := os.Lstat(clean)
	if err != nil {
		return "", fmt.Errorf("inspect sysfs root %q: %w", clean, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("sysfs root %q is not a directory", clean)
	}
	return clean, nil
}

func collectDirectory(
	ctx context.Context,
	sysRoot, directory string,
	bus Bus,
	observedAt time.Time,
) ([]Device, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s devices %q: %w", bus, directory, err)
	}
	var devices []Device
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if bus == BusCPU && !isCPUDevice(entry.Name()) {
			continue
		}
		sourcePath := filepath.Join(directory, entry.Name())
		resolved, err := filepath.EvalSymlinks(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("resolve sysfs device %q: %w", sourcePath, err)
		}
		if !within(sysRoot, resolved) {
			return nil, fmt.Errorf("sysfs device %q resolves outside %q", sourcePath, sysRoot)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("inspect sysfs device %q: %w", sourcePath, err)
		}
		if !info.IsDir() {
			continue
		}
		device, err := readDevice(sysRoot, resolved, sourcePath, entry.Name(), bus, observedAt)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func readDevice(
	sysRoot, resolved, sourcePath, address string,
	bus Bus,
	observedAt time.Time,
) (Device, error) {
	attributes := make(map[string]string)
	for _, name := range []string{"vendor", "device", "class", "product", "name", "model", "modalias"} {
		value, err := readAttribute(filepath.Join(resolved, name))
		if err != nil {
			return Device{}, fmt.Errorf("read %s for %q: %w", name, sourcePath, err)
		}
		attributes[name] = value
	}
	driver, err := linkedBase(sysRoot, filepath.Join(resolved, "driver"))
	if err != nil {
		return Device{}, fmt.Errorf("read driver for %q: %w", sourcePath, err)
	}
	module, err := linkedBase(sysRoot, filepath.Join(resolved, "driver", "module"))
	if err != nil {
		return Device{}, fmt.Errorf("read module for %q: %w", sourcePath, err)
	}
	name := firstNonempty(attributes["product"], attributes["name"], attributes["model"], attributes["modalias"])
	device := Device{
		Bus: bus,
		ID: Identifier{
			Vendor: trimHexPrefix(attributes["vendor"]), Product: trimHexPrefix(attributes["device"]),
			Class: trimHexPrefix(attributes["class"]), Address: address,
		},
		Name: name, Driver: driver, Presence: Present,
		Provenance: []domain.Provenance{{
			Kind: domain.SourceDevice, Source: sourcePath,
			Detail: "device and bound driver observed in sysfs", ObservedAt: observedAt,
		}},
	}
	if module != "" {
		device.Modules = []string{module}
	}
	return device, nil
}

func readAttribute(path string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		// Bus topology commonly uses attribute-like names such as "device" for
		// symlinks. Optional scalar evidence is read only from regular files.
		return "", nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAttributeSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxAttributeSize {
		return "", fmt.Errorf("attribute exceeds %d bytes", maxAttributeSize)
	}
	return strings.TrimSpace(string(data)), nil
}

func linkedBase(sysRoot, path string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("link is not a symlink")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if !within(sysRoot, resolved) {
		return "", fmt.Errorf("link resolves outside sysfs")
	}
	return filepath.Base(resolved), nil
}

func within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func isCPUDevice(name string) bool {
	if !strings.HasPrefix(name, "cpu") || len(name) == 3 {
		return false
	}
	for _, character := range name[3:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func trimHexPrefix(value string) string {
	return strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
