# Gentooling consumer requirements

## Scope

This document initially assessed Arise `master` at commit `f190725` as the
source of libraries for `github.com/airencracken/gentooling`. It was rechecked
against Arise `b869c74` and Gentooling `4bc9357`. It describes what Maize needs
as a Go-library consumer. It does not prescribe an Arise refactor and does not
propose subprocess integration.

The central ownership rule is:

> Gentooling reports Gentoo package-manager truth. Maize combines that truth
> with hardware and operator policy to make kernel decisions.

Maize cannot import Arise's current packages directly because they live beneath
`github.com/airencracken/arise/internal`. Candidate code therefore needs a
public home in Gentooling with consumer-oriented contracts.

## Summary

Arise already contains strong candidates for:

- Installed package and recorded USE inventory.
- Active profile stacking and profile USE policy.
- Effective `/etc/portage` configuration.
- Atom parsing and matching.
- Repository metadata records and queries.
- World and system set loading.
- Deterministic package-state snapshots.

The main missing capability is structured discovery of package-to-kernel
requirements. Repository cache metadata can reveal that an ebuild inherited
`linux-info.eclass`, but it does not contain the values assembled in
`CONFIG_CHECK`, nor the USE-conditional behavior executed by
`check_extra_config` during an ebuild phase.

The current code also needs a consumer boundary around paths, environment,
diagnostics, snapshot consistency, and storage. Maize should not receive a
`*badger.DB`, process environment, hard-coded host paths, mutable internal maps,
or silently incomplete VDB results.

## Recheck status

Gentooling now exists as a single Go module and root package at
`github.com/airencracken/gentooling`. It has no release tags yet; Arise consumes
the commit through pseudo-version
`v0.0.0-20260730194649-4bc935797792`.

The first extraction milestone implements:

- `PackageID`, `ParsePackageID`, `CP`, and `CPV`.
- Root-relative `SystemPaths` and `DefaultSystemPaths`.
- `ReadInstalled(context.Context, SystemPaths, InstalledOptions)`.
- Installed identity, EAPI, recorded enabled and declared USE, dependencies,
  build metadata, and optional contents.
- `AllowPartial` and `RequireComplete` integrity modes.
- Typed issue codes and errors for malformed, interrupted, corrupt, unreadable,
  and invalid installed records.
- Deterministic inventory and issue ordering.
- Context cancellation between scanned records.

Arise now imports Gentooling and delegates `internal/vdb` scans to it.
`ScanWithIssues` exposes the typed diagnostics inside Arise. The older `Scan`
and `ScanResolverState` wrappers deliberately use partial mode and discard the
issues, so existing Arise callers retain their previous silent-partial
semantics. This does not block Maize because Maize should call Gentooling
directly with `RequireComplete`.

The current implementation closes the first, narrow installed-inventory slice
of requirements 1, 2, and 6 below. Effective configuration, profiles, USE
evaluation, selections, atoms, repositories, snapshots, and kernel requirements
have not yet been extracted.

### Recheck findings: needed now

#### Validate `IntegrityMode`

`ReadInstalled` treats every value other than `RequireComplete` as partial
mode. An invalid enum value such as `IntegrityMode(255)` should return
`ErrInvalidData`, otherwise a configuration or decoding bug can silently
downgrade a strict evidence request.

#### Detect concurrent mutation

`ErrConcurrentMutation` is declared but unused. The scan performs independent
directory and file reads without a before/after generation check or shared
package-manager lock. A package merge can therefore produce a mixture of
states without a diagnostic. This remains a blocker for treating a scan as
strict generation evidence.

#### Do not read `CONTENTS` when excluded

`CONTENTS` is correctly required as the commit marker, but
`readInstalledRecord` reads it into memory with the other required values even
when `IncludeContents` is false. It only suppresses copying the already-read
value into the result. Resolver and Maize inventory scans should validate the
regular file without reading the potentially large payload.

#### Reject path-like package identities

`ParsePackageID` permits category strings accepted by
`^[A-Za-z0-9+_.-]+$`, including `.` and `..`. These are not safe public package
identities and can become traversal components if a consumer later joins a
`PackageID` to a root. The parser should reject dot path components explicitly,
and its adversarial tests should include them.

#### Define optional-file symlink policy

Required VDB files are rejected when they are symlinks, but optional files are
read with `os.ReadFile` without an `Lstat`, allowing an optional metadata
symlink to escape the VDB record. Gentooling should either reject all metadata
symlinks or document and safely constrain which ones are valid. Maize should
not ingest evidence through an unbounded path escape.

#### Add declared USE structure

`DeclaredUse []string` preserves raw `IUSE` tokens such as `+ssl` and `-foo`.
This is lossless enough for the initial extraction, but it leaves every
consumer to parse default state. The proposed `UseDeclaration{Name, Default}`
model is still needed before Maize compares recorded and effective USE.

#### Stabilize release consumption

Arise currently depends on a Gentooling pseudo-version and Gentooling has no
tags. That is appropriate during initial extraction, but Maize should begin
consuming it only after the required API slice is tagged or an explicit
pre-v1 compatibility policy exists.

### Recheck findings: likely soon

#### Separate module stability from one large root package

There is currently one package containing paths, identities, issues, and VDB
scanning. That is acceptable for the foundation. As configuration, profiles,
repository access, atoms, and kernel requirement capture arrive, Gentooling
should expose cohesive packages or deliberately define one facade so Maize
does not inherit a large, tightly coupled root namespace.

#### Complete error-category coverage

The installed API preserves filesystem errors and exposes integrity sentinels,
which is a strong base. `ErrConcurrentMutation` needs real semantics, and
not-found behavior will need a consistent contract when query APIs arrive.
`Issue.Cause` is useful in-process but should not itself become a serialized
schema; snapshots need stable codes and explicit fields.

### Recheck findings: still missing

No Gentooling APIs currently exist for:

- Effective Portage configuration.
- Profile graphs and profile policy.
- USE evaluation or provenance.
- World/system selections.
- Gentoo dependency atoms and matching.
- Consistent system snapshots or fingerprints.
- Repository metadata/querying.
- Installed file ownership.
- Out-of-tree kernel module classification.
- Structured `linux-info`/Kconfig requirements.

The extraction order at the end of this document remains valid. Installed
inventory is now substantially complete once the strictness and filesystem
issues above are addressed.

## Needed now

### 1. Installed package inventory

#### Why Maize needs it

The installed CPV, slot, repository, EAPI, declared USE flags, enabled USE
flags, and dependency metadata are the authoritative record of package
features currently present on the machine.

#### Candidate Arise code

- `internal/vdb/vdb.go`
- `vdb.Package`
- `vdb.ScanResolverState`
- `metadata.ParseCPV`

The existing model already retains `USE`, `IUSE`, `DEPEND`, `RDEPEND`,
`BDEPEND`, `IDEPEND`, `PDEPEND`, slot/subslot, repository, and EAPI.

#### Proposed API

```go
package gentooling

type PackageID struct {
    Category   string
    Name       string
    Version    string
    Slot       string
    Subslot    string
    Repository string
}

type UseState struct {
    Declared []UseDeclaration
    Enabled  []string
}

type UseDeclaration struct {
    Name    string
    Default UseDefault
}

type InstalledPackage struct {
    ID           PackageID
    EAPI         string
    Use          UseState
    Dependencies DependencyMetadata
    Build        BuildMetadata
}

type InstalledInventory struct {
    Packages []InstalledPackage
    Issues   []Issue
}

func ReadInstalled(
    ctx context.Context,
    paths SystemPaths,
    options InstalledOptions,
) (InstalledInventory, error)
```

The result must be sorted and own all returned slices and maps.

#### Error behavior required

The current scanner silently skips malformed CPVs, incomplete package records,
failed per-file reads, and empty required values. That behavior protects the
resolver from interrupted merges, but Maize must know that its evidence is
incomplete.

`ReadInstalled` should:

- Return an error when the VDB root or category cannot be read.
- Return valid records plus typed issues for ignored partial, malformed, or
  concurrently changing entries.
- Distinguish an interrupted/uncommitted record from corrupt committed
  metadata.
- Preserve `errors.Is` support for permission, not-found, invalid-data, and
  concurrent-mutation categories.
- Never return a package with silently substituted zero values after a read or
  integer parse failure.

Strict mode should promote any corrupt committed record to an error. Lenient
mode may return a partial inventory with issues. Maize should use strict mode
for final generation and may use lenient mode for an explanatory inspection.

#### Coupling to remove from the public contract

- Direct `os` and absolute-path traversal is acceptable behind the API, but the
  caller must provide a complete `SystemPaths`.
- The public result must not expose VDB file contents or Arise internal types.
- No package-manager command may be executed.

### 2. Explicit system layout

#### Why Maize needs it

Maize must analyze the live root, an alternate root, or fixtures without
accidentally reading host `/usr`, `/etc`, or `/var`.

#### Candidate Arise code

- `portage.LoadConfig`
- `portage.LoadEffectiveConfigWithEnvironment`
- `profile.LoadProfile`
- `vdb.ScanResolverState`
- `world.LoadWorld`

`LoadEffectiveConfigWithEnvironment` is already a useful step because it can
take the command environment as data. It still reads
`/usr/share/portage/config/make.globals` directly and infers repository layout
from paths.

#### Proposed API

```go
type SystemPaths struct {
    Root            string
    ConfigRoot      string
    VDB             string
    World           string
    MakeGlobals     string
    Repositories    []RepositoryPath
    ActiveProfile   string
}

type LoadOptions struct {
    Environment []string
    Consistency ConsistencyMode
}

func DiscoverSystemPaths(
    ctx context.Context,
    root string,
) (SystemPaths, error)

func LoadSystem(
    ctx context.Context,
    paths SystemPaths,
    options LoadOptions,
) (*System, error)
```

`DiscoverSystemPaths` is convenience. `LoadSystem` is authoritative and must
not consult paths or environment absent from its arguments.

#### Error behavior required

- A missing optional user configuration file is not an error.
- A missing configured profile target, unreadable make.globals, invalid parent
  graph, or repository escape is an error.
- Errors identify the logical input and path without requiring string parsing.
- Context cancellation is returned promptly during large scans.

### 3. Effective configuration and USE evaluation

#### Why Maize needs it

Recorded installed USE describes what was built. Effective Portage policy
describes what would be built now. Maize needs both to detect drift and to
explain whether a kernel capability is required by an installed feature,
new policy, profile force/mask, or operator override.

#### Candidate Arise code

- `internal/portage/portage.go`
- `portage.Config`
- `EffectiveUseForStability`
- `ExplicitUseOverride`
- `UseMaskedFor`
- `UseForcedFor`
- `PackageUseFor`
- `internal/profile/profile.go`

The reduction logic appears mature, including ordered package rules, stable
force/mask policy, USE_EXPAND, and explicit command-environment injection.
The current public shape is too low-level: callers receive mutable maps and
must independently apply IUSE filtering and defaults.

#### Proposed API

```go
type PackageContext struct {
    ID       PackageID
    EAPI     string
    Keywords []string
    IUse     []UseDeclaration
}

type UseDecision struct {
    Name       string
    Enabled    bool
    Declared   bool
    Forced     bool
    Masked     bool
    StableOnly bool
    Sources    []PolicySource
}

type UseEvaluation struct {
    Package   PackageID
    Decisions []UseDecision
}

func (s *System) EvaluateUse(
    ctx context.Context,
    pkg PackageContext,
) (UseEvaluation, error)
```

The API should apply IUSE defaults and filtering itself. `Sources` must retain
ordered provenance such as profile layer, make.conf, package.use file and line,
force/mask rule, and explicit environment override.

#### Missing functionality

Arise calculates the effective result but generally does not retain
file-and-line provenance through the final reduction. Maize can begin without
full line numbers, but it needs at least the policy layer and source path to
produce trustworthy explanations.

### 4. Active profile, world, and system intent

#### Why Maize needs it

Package installation alone is weak evidence of an active runtime requirement.
World membership, system-set membership, and dependency-only status help Maize
rank a package-derived kernel requirement without pretending that an installed
utility proves a filesystem or protocol is in use.

#### Candidate Arise code

- `internal/profile.ProfileInfo.SystemSet`
- `internal/world.LoadWorld`
- Atom matching from `internal/atom`
- Installed dependency metadata from `internal/vdb`

#### Proposed API

```go
type SelectionReason string

const (
    SelectedWorld      SelectionReason = "world"
    SelectedSystem     SelectionReason = "system"
    SelectedDependency SelectionReason = "dependency"
    SelectedUnknown    SelectionReason = "unknown"
)

type Selection struct {
    Atom   string
    Source PolicySource
}

func (s *System) Selections(ctx context.Context) ([]Selection, error)

func (s *System) SelectionReasons(
    ctx context.Context,
    installed []InstalledPackage,
) (map[PackageID][]SelectionReason, error)
```

The first API is necessary now. Computing complete dependency provenance may
start conservatively and improve later.

### 5. Stable atoms and package identities

#### Why Maize needs it

Maize rules will target versioned packages, slots, repositories, and USE
conditions. It must not implement Gentoo atom semantics.

#### Candidate Arise code

- `internal/atom`
- `installedquery`
- `portage.PackageAtomMatches`

#### Proposed API

```go
type Atom struct { /* opaque parsed representation */ }

func ParseAtom(value string) (Atom, error)
func (a Atom) String() string
func (a Atom) Matches(pkg PackageID, use UseState) (bool, error)
```

The parsed representation should be immutable or behave as immutable. An
invalid atom is a typed input error. No-match is `false, nil`, not an error.

### 6. Snapshot consistency and diagnostics

#### Why Maize needs it

A package transaction may mutate the VDB or configuration during inspection.
Combining records read at different moments can yield an internally
inconsistent kernel recommendation.

#### Candidate Arise code

- `internal/packagestate`
- `internal/oplock`
- Deterministic ordering and fingerprints already used by snapshots

`packagestate.Snapshot` is useful precedent, but it exposes repository records
and depends directly on `*badger.DB`; it is not a complete effective-system
snapshot.

#### Proposed API

```go
type SystemSnapshot struct {
    Schema      uint
    Fingerprint string
    CapturedAt  time.Time
    Installed   []InstalledPackage
    Profile     Profile
    Selections  []Selection
    Config      EffectiveConfig
    Issues      []Issue
}

func (s *System) Snapshot(
    ctx context.Context,
    options SnapshotOptions,
) (SystemSnapshot, error)
```

The snapshot contract should state whether it:

- Acquires the same read/transaction lock used by package operations.
- Detects before/after changes and returns `ErrConcurrentMutation`.
- Or explicitly provides best-effort consistency with issues.

Maize needs one strict mode for generation.

## Likely soon

### 7. Structured package kernel requirements

#### Why Maize needs it

This is the bridge from package and USE state to Kconfig constraints. Examples
include container runtimes, filesystems, virtualization tooling, networking
services, security features, and out-of-tree modules.

#### Candidate Arise code

- `metadata.PackageMetadata.INHERITED` can identify `linux-info`.
- `metadata.PackageMetadata.DEFINED_PHASES` identifies relevant phase hooks.
- Ebuild and eclass parsing in `internal/ebuild` and `internal/eclass` may help
  locate candidate declarations.
- Arise's package phase environment can evaluate USE-conditional ebuild logic.

#### Missing functionality

There is no structured Arise API for:

- `CONFIG_CHECK` expressions.
- `WARNING_FUTURE_KERNEL_CONFIG`.
- `ERROR_*` kernel configuration policy from `linux-info.eclass`.
- The USE conditions under which a kernel requirement applies.
- Whether a symbol must be built-in or may be a module.
- The help or rationale presented by the ebuild/eclass.

Static parsing alone will not be reliable because ebuilds can construct these
values through shell execution and USE conditionals. Conversely, executing
arbitrary build phases merely to inspect requirements is too broad.

#### Proposed API

```go
type KernelRequirementKind string

const (
    KernelRequired    KernelRequirementKind = "required"
    KernelRecommended KernelRequirementKind = "recommended"
    KernelForbidden   KernelRequirementKind = "forbidden"
)

type KernelSymbolRequirement struct {
    Symbol      string
    Acceptable  []KernelSymbolState
    Kind        KernelRequirementKind
    Conditions  []UseCondition
    Rationale   string
    Source      PolicySource
    Confidence  Confidence
}

type PackageKernelRequirements struct {
    Package      PackageID
    Requirements []KernelSymbolRequirement
    Issues       []Issue
}

func (s *System) KernelRequirements(
    ctx context.Context,
    pkg PackageID,
    use UseState,
) (PackageKernelRequirements, error)
```

The preferred Gentooling implementation is a constrained, read-only metadata
evaluation or structured capture point in the existing Arise phase machinery.
It should capture kernel-policy variables and `linux-info` checks without
running build, install, or mutation behavior. It must declare which EAPIs and
eclass patterns it supports. Unsupported dynamic behavior becomes an issue,
not an invented requirement.

Maize will supplement this source with reviewed Maize capability rules. It
must retain whether a requirement came from Gentoo metadata or Maize policy.

### 8. Repository metadata without storage leakage

#### Why Maize needs it

For migration and package-impact analysis, Maize may need the installed
version's current ebuild metadata and the selected upgrade candidate. This is
especially relevant when kernel requirements change with a package upgrade.

#### Candidate Arise code

- `internal/metadata.PackageMetadata`
- `internal/ingest`
- `internal/repositoryquery`
- `internal/walker`

#### Current coupling

Repository queries take `*badger.DB`, expose Arise metadata pointers, and fall
back to `runtime.GOARCH` when `ARCH` is absent. Badger and runtime architecture
must not be part of Gentooling's public consumer contract.

#### Proposed API

```go
type PackageMetadata struct {
    ID            PackageID
    EAPI          string
    IUse          []UseDeclaration
    Dependencies  DependencyMetadata
    Inherited     []string
    DefinedPhases []string
    Complete      bool
}

type Repository interface {
    Metadata(ctx context.Context, id PackageID) (PackageMetadata, error)
    BestVisible(ctx context.Context, atom Atom) (PackageMetadata, error)
}
```

Gentooling may use Badger internally. Consumers should receive a repository
handle and immutable values. Architecture must come from loaded effective
configuration or explicit options, never the Go runtime as a silent policy
fallback.

### 9. Installed file ownership

#### Why Maize may need it

File ownership can connect active services, helpers, firmware, kernel modules,
or configuration to an installed package. It is supporting provenance, not a
primary kernel requirement.

#### Candidate Arise code

- `packagequery.Owners`
- VDB `CONTENTS` parsing

#### Proposed API

```go
type OwnedPath struct {
    Path     string
    Packages []PackageID
}

func (s *System) Owners(
    ctx context.Context,
    paths []string,
) ([]OwnedPath, error)
```

Exact paths and basenames must be distinct query modes; implicit suffix or
basename guessing is unsuitable for machine evidence.

### 10. Out-of-tree kernel module packages

#### Why Maize needs it

An optimal in-tree configuration is insufficient when installed packages build
external modules. Maize must highlight target-kernel compatibility, required
in-tree prerequisites, module signing, and rebuild requirements.

#### Candidate Arise code

- Installed package metadata and `INHERITED` eclasses.
- Package contents for installed `.ko` files.
- Repository metadata for the target version.

#### Missing functionality

Arise has no consumer model classifying packages as external kernel modules or
describing supported kernel ranges and prerequisite Kconfig symbols.

#### Proposed API

```go
type KernelModulePackage struct {
    Package       PackageID
    Modules       []string
    Eclasses      []string
    KernelRange   VersionConstraint
    Requirements  []KernelSymbolRequirement
    Confidence    Confidence
}

func (s *System) KernelModulePackages(
    ctx context.Context,
) ([]KernelModulePackage, error)
```

Some classification may ultimately remain a Maize rule because package
metadata does not express all compatibility constraints structurally.

## Speculative

### 11. Package dependency provenance graph

A full installed dependency graph could explain why a kernel-relevant package
is installed and whether it is still reachable from world/system. Arise's
resolver and graph packages are candidates, but Maize can initially use simpler
selection categories and avoid importing resolver internals.

Possible API:

```go
func (s *System) WhyInstalled(
    ctx context.Context,
    pkg PackageID,
) ([]DependencyPath, error)
```

### 12. Planned transaction impact

Maize could eventually accept a prospective Arise transaction plan and report
kernel requirements introduced or removed by package/USE changes before the
transaction executes.

Possible API:

```go
func RequirementsForPlan(
    ctx context.Context,
    snapshot SystemSnapshot,
    plan PackagePlan,
) (KernelRequirementDelta, error)
```

This should use a stable Gentooling plan model, not Arise CLI JSON unless that
JSON itself becomes the versioned library contract.

### 13. Package feature descriptions

Human-readable explanations for why a USE flag implies a kernel capability
would improve reports. Ebuild metadata does not consistently encode that
relationship. A reviewed knowledge base shared by Gentooling and Maize might
eventually expose descriptions and citations, but it should not block initial
generation.

## Cross-cutting public contracts

### Context

All filesystem scans, repository queries, and metadata evaluation APIs should
accept `context.Context`. Current candidate functions do not. Cancellation
must not return a result that appears complete.

### Issues versus errors

Use errors when the requested result cannot be trusted. Use issues when a
well-defined partial result remains useful.

```go
type IssueSeverity string

const (
    IssueInfo    IssueSeverity = "info"
    IssueWarning IssueSeverity = "warning"
    IssueError   IssueSeverity = "error"
)

type Issue struct {
    Code     string
    Severity IssueSeverity
    Path     string
    Package  *PackageID
    Message  string
    Cause    error
}
```

Issue codes are stable machine contracts. Messages are operator-facing and may
evolve. Returned errors wrap stable sentinels or typed errors so callers use
`errors.Is`/`errors.As`, not substring matching.

### Not found

- Query APIs: typed `ErrNotFound`.
- Collection APIs: empty collections are successful when the collection
  legitimately exists but has no members.
- Optional configuration files: absence is represented in provenance, not
  necessarily an error.
- Required roots and configured targets: absence is an error.

Avoid the current mixture of `nil, nil`, empty string, empty collection, and
formatted error for semantically similar misses.

### Immutability and concurrency

Public snapshots and evaluations should behave as immutable values. Arise's
current `portage.Config` exposes maps and slices and has a package-global
unbounded `sync.Map` atom cache. Gentooling should keep caches private,
bounded or instance-owned, and must not expose state that callers can mutate
concurrently.

### Filesystem access

A broad virtual filesystem abstraction is not required initially. Explicit,
root-aware `SystemPaths` plus temp-directory fixtures are sufficient. Every
host path must derive from those inputs. Public APIs must not silently read:

- `/etc/portage`
- `/var/db/pkg`
- `/usr/share/portage`
- the process environment
- the Go runtime architecture

### Storage

Badger is a Gentooling implementation detail. Maize should not open Arise's
database, depend on key prefixes, or receive `*badger.DB`. Repository handles
own lifecycle explicitly and support deterministic close behavior.

### Side effects

All APIs proposed here are read-only. They must not:

- Modify VDB, world, profile, or Portage configuration.
- Acquire a write-oriented global package-manager operation unless required
  solely to guarantee a documented consistent snapshot.
- Execute ebuild build/install phases.
- Invoke package-manager or query commands.
- Depend on current working directory.

## Recommended extraction order

1. Public atom and identity value types.
2. Root-aware installed inventory with typed issues.
3. Root-aware effective configuration and profile loading with explicit
   environment.
4. USE evaluation including IUSE defaults and policy provenance.
5. World/system selections and a strict consistent system snapshot.
6. Repository metadata behind a storage-neutral interface.
7. Structured kernel-requirement capture.
8. File ownership and out-of-tree module classification.

This order lets Maize begin trustworthy package inventory and rule-based
capability mapping before the harder dynamic ebuild requirement extraction is
available.
