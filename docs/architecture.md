# Architecture sketch

## Data flow

```text
old Kconfig and .config ----------+
target Kconfig -------------------+
hardware and peripheral history --+--> requirements --> resolver
Gentoo and Portage state ---------+                       |
operator profiles ----------------+                       v
                                                   candidate config
                                                          |
                                                   target Kconfig
                                                          |
                                                          v
                                             .config and audit report
```

## Packages

### `internal/domain`

Stable vocabulary shared by the application: capabilities, evidence,
requirements, desired states, confidence, provenance, conflicts, and decisions.
This package must not know how evidence was collected.

### `internal/profile`

Loads versioned TOML profiles, resolves inheritance, validates policy, and
compiles stable capabilities into domain requirements.

### `internal/gentooling`

Adapts `github.com/airencracken/gentooling` libraries into Maize's domain
vocabulary. Gentooling provides general Gentoo package data extracted from
Arise: installed packages, effective USE flags, Portage profiles, package
configuration, and ebuild/eclass metadata.

The adapter prevents Gentooling's package model from leaking into the Kconfig
resolver and gives Maize stable contract fixtures while the libraries are
extracted. It is intentionally not a cross-distribution package-provider
interface.

This integration is exclusively in-process Go library use. The package may not
execute `arise`, `equery`, `q`, `portageq`, or any other package-query command.
Missing functionality is implemented and exported by Gentooling, normally by
extracting it from Arise, then consumed by Maize. The architecture test for
this package rejects imports of `os/exec`.

### `internal/gentoo`

Owns kernel-specific Gentoo integration not provided by Arise: selected kernel
sources, distribution kernel configuration, initramfs policy, boot integration,
and out-of-tree module compatibility.

### `internal/inventory`

Collects current hardware and operating state and merges it with persisted
peripheral history. Collection and persistence remain separate so fixtures can
exercise resolution without access to host hardware.

### `internal/kconfig`

Indexes symbol definitions, prompts, help, types, defaults, ranges,
dependencies, choices, `select`, `imply`, and source locations. It explains
semantic differences but delegates final evaluation to kernel-provided Kconfig
tools.

### `internal/resolve`

Combines requirements, reports contradictions, proposes symbol states, and
retains complete provenance. Resolution is deterministic and transactional: an
error cannot return a partial plan.

### `internal/report`

Produces deterministic human-readable output and versioned JSON. JSON schemas
are public compatibility contracts.

### `internal/kernel`

Runs target Kconfig commands in isolated output directories, captures resulting
changes, and never mutates the source tree or operator's input configuration.

### `cmd/maize`

Defines the CLI and maps commands to application use cases. Business logic does
not live in the CLI package.

## Initial domain rules

- A capability is stable operator intent such as `containers`, `root-ext4`, or
  `intel-wifi`, not a Kconfig symbol.
- Gentooling reports package truth; Maize decides what that truth means for a
  kernel configuration.
- Package-manager access is through Gentooling Go APIs only, never subprocess
  output.
- Requirements from multiple sources accumulate rather than overwrite.
- A required and prohibited state for the same capability is a conflict.
- Stronger evidence may determine a recommendation, but it cannot silently
  override an explicit prohibition.
- Output ordering is stable to support review and reproducible tests.
- Failed resolution returns no decisions.

## Testing strategy

- Unit tests cover validation and individual resolution rules.
- Integration tests cover profile-to-requirement-to-plan flows and kernel tool
  execution using fixtures.
- Property tests exercise order independence, idempotence, and determinism.
- Mutation testing is expected for resolver and validation packages.
- Schema-validation tests protect TOML inputs and JSON output contracts.
- API-contract tests protect exported JSON structures and CLI behavior.
- Atomicity tests prove failed runs leave no partial configuration or report.
- Adversarial tests cover cycles, malformed metadata, oversized inputs,
  duplicate evidence, path traversal, hostile package metadata, and command
  argument boundaries.
- Route-existence tests are not applicable unless Maize later exposes a network
  service; the CLI command tree receives the equivalent coverage.
