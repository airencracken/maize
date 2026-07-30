# Product plan

## Product statement

Maize generates and migrates optimized Gentoo kernel configurations using
hardware evidence, Portage state, USE flags, and explicit operator profiles,
while explaining every consequential change.

It is intentionally Gentoo-specific. Portage, Gentoo profiles, ebuild metadata,
USE flags, selected kernel sources, initramfs tooling, and out-of-tree modules
are core inputs rather than optional distribution adapters.

Maize is intended to consume reusable Go libraries extracted from Arise, the
companion emerge replacement. Arise owns general Portage and package
functionality; Maize owns the translation from Gentoo system intent to kernel
capabilities and Kconfig decisions. Until those libraries have stable package
APIs, the dependency is represented as an architectural boundary rather than a
guessed import path.

## Goals

1. Preserve operator intent while moving between kernel releases.
2. Produce the smallest defensible configuration for an explicit operating
   profile.
3. Keep current hardware, known peripherals, installed packages, and enabled
   USE-flag functionality working.
4. distinguish required, recommended, optional, and prohibited capabilities.
5. Explain every material configuration decision with provenance and
   confidence.
6. Ask the operator only about choices that cannot be resolved safely.
7. Use the target kernel's own Kconfig implementation as the final authority.

## Non-goals

- Supporting distributions other than Gentoo.
- Reimplementing all Kconfig evaluation semantics.
- Treating an unloaded module as proof that its functionality is unnecessary.
- Inferring kernel requirements from USE-flag names alone.
- Silently choosing between conflicting operator policies.
- Building, installing, or booting a kernel in the first milestone.

## Evidence

Maize combines evidence from:

- The existing `.config` and old kernel Kconfig tree.
- The target kernel Kconfig tree.
- PCI, USB, HID, SCSI, NVMe, platform, ACPI, I2C, SPI, Thunderbolt, input,
  block, filesystem, network, CPU, firmware, and virtualization state.
- Loaded modules and module aliases.
- Root, boot, encryption, RAID, LVM, device-mapper, and initramfs state.
- A persisted history of removable or intermittently connected peripherals.
- `/var/db/pkg` installed package metadata and recorded USE flags.
- The active Gentoo profile and effective Portage configuration.
- Kernel requirements explicitly expressed by ebuilds and eclasses.
- Curated, reviewed, versioned package capability rules.
- Explicit operator profiles and overrides.

Evidence strength, provenance, and observation time must be retained. A current
root filesystem is stronger evidence than an installed filesystem utility.

## Profiles

Profiles express stable operator intent, not raw `CONFIG_*` values. They use
TOML and may extend other profiles.

```toml
schema = "maize.profile/v1"
name = "gaming-workstation"
extends = ["workstation", "hardened"]

[optimize]
size = "balanced"
attack_surface = "reduced"
build_time = "balanced"

[hardware]
current_machine = "required"
previously_observed = "include"
removable_devices = "common"
portability = "this-machine"

[portage]
installed_packages = "required"
respect_use_flags = true
unknown_kernel_requirements = "warn"

[boot]
initramfs = true
root_storage = "builtin"
root_filesystem = "builtin"
recovery_support = true

[capabilities]
gaming = true
audio = true
bluetooth = true
containers = false
virtualization_host = true
```

Initial built-in profiles:

- `minimal`
- `workstation`
- `laptop`
- `server`
- `virtualization-host`
- `virtualization-guest`
- `container-host`
- `hardened`
- `developer`

Profile layering order:

1. Built-in baseline.
2. Gentoo profile policy.
3. Operator-selected profiles.
4. Machine inventory.
5. Explicit operator overrides.

Conflicts are reported; precedence must not conceal contradictions.

## Decisions and reporting

Every proposed symbol value records:

- The capability it implements.
- Whether it is required, recommended, optional, or prohibited.
- Why it is built-in, modular, or disabled.
- All supporting and conflicting evidence.
- The profile, package, USE flag, device, or dependency that introduced it.
- Confidence and any unresolved assumptions.
- Changes made by target Kconfig validation.

Reports are plain text for operators and versioned JSON for automation.

## Commands

```text
maize inspect
    Collect and explain machine and Portage requirements.

maize generate
    Generate a configuration from profiles and evidence.

maize migrate
    Preserve old intent, explain target Kconfig changes, and optionally
    re-optimize for current evidence.

maize check
    Check the running or selected kernel against the Gentoo system.

maize impact
    Explain which hardware, packages, USE-flag features, or profile
    capabilities a proposed configuration may break.

maize observe
    Add current hardware and peripherals to a persistent inventory.
```

## Milestones

### 1. Explain an existing migration

- Load two kernel source trees and an old `.config`.
- Run target `olddefconfig` in an isolated output directory.
- Classify added, removed, renamed, replaced, and behavior-changing symbols.
- Explain target-induced value changes.
- Emit deterministic text and JSON reports.

### 2. Model profiles and constraints

- Parse and validate versioned TOML profiles.
- Resolve inheritance without cycles.
- Compile capabilities into requirements.
- Report conflicts and unresolved choices.

### 3. Inventory a Gentoo machine

- Record hardware, storage, filesystems, modules, firmware, and boot state.
- Persist peripheral history with timestamps.
- Distinguish boot-critical, current, historical, and policy evidence.

### 4. Model Portage requirements

- Integrate Arise libraries for installed package metadata, effective USE
  flags, profile state, and ebuild/eclass metadata.
- Extract explicit kernel requirements from Arise-provided package data where
  safe.
- Add reviewed package capability rules.
- Implement configuration impact analysis.

Maize must not grow a second general-purpose Portage implementation while these
capabilities are being extracted into Arise libraries. Narrow temporary
adapters are acceptable only when they preserve a replaceable boundary and are
covered by contract fixtures.

### 5. Generate and optimize

- Combine profiles, hardware, Portage, and existing intent.
- Choose built-in, module, or disabled states.
- Validate through target Kconfig.
- Minimize configuration without discarding unexplained capabilities.

### 6. Operational hardening

- Support interrupted runs and atomic output.
- Add stable schemas, fixtures from real kernel versions, and compatibility
  guarantees.
- Provide shell completion and Gentoo packaging.

## Safety principles

- Analysis is read-only unless an output path is explicit.
- Generated files are written atomically.
- Kernel sources and the live `.config` are never modified in place.
- Building and installation require separate explicit commands.
- Uncertainty produces a warning or question, never a silent removal.
- Target Kconfig tools validate every generated configuration.
