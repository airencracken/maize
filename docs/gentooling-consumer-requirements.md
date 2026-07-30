# Gentooling consumer requirements

## Scope

This document initially assessed Arise `master` at commit `f190725` as the
source of libraries for `github.com/airencracken/gentooling`. It was rechecked
against Arise `b869c74` and Gentooling `4bc9357`, then updated for the
Gentooling `v0.1.0` release at `be14afc` and Arise adoption at `c83a51c`. It
describes what Maize needs as a Go-library consumer. It does not prescribe an
Arise refactor and does not propose subprocess integration.

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
`github.com/airencracken/gentooling`. Version `v0.1.0` is its first public
release. Arise consumes that tagged version directly.

The first extraction milestone implements:

- `PackageID`, `ParsePackageID`, `CP`, and `CPV`.
- Root-relative `SystemPaths` and `DefaultSystemPaths`.
- `ReadInstalled(context.Context, SystemPaths, InstalledOptions)`.
- Installed identity, EAPI, recorded enabled and declared USE, dependencies,
  build metadata, and optional contents.
- Structured `UseDeclaration` values retaining enabled, disabled, and
  unspecified IUSE defaults.
- `AllowPartial` and `RequireComplete` integrity modes.
- Typed issue codes and errors for malformed, interrupted, corrupt, unreadable,
  invalid, and concurrently changing installed records.
- Validated integrity and worker options.
- Bounded concurrent record reads with deterministic output.
- Symlink-safe metadata reads and opt-in `CONTENTS` payload loading.
- Deterministic inventory and issue ordering.
- Context cancellation between scanned records.
- A documented pre-1.0 semantic-versioning and deprecation policy.

Arise now imports Gentooling and delegates `internal/vdb` scans to it.
`ScanWithIssues` exposes the typed diagnostics inside Arise. The older `Scan`
and `ScanResolverState` wrappers deliberately use partial mode and discard the
issues, so existing Arise callers retain their previous silent-partial
semantics. This does not block Maize because Maize should call Gentooling
directly with `RequireComplete`.

The `v0.1.0` implementation closes the first, narrow installed-inventory slice
of requirements 1, 2, and 6 below. Effective configuration, profiles, USE
evaluation, selections, atoms, repositories, full-system snapshots, and kernel
requirements have not yet been extracted.

### Findings resolved by `v0.1.0`

#### Validate `IntegrityMode`

Resolved. `ReadInstalled` rejects unknown integrity modes and negative worker
counts with `ErrInvalidData`.

#### Detect concurrent mutation

Resolved for installed inventory through before/after filesystem snapshots of
the VDB root, categories, records, and read metadata. Observed changes produce
`IssueConcurrentMutation` wrapping `ErrConcurrentMutation`; strict mode
promotes the issue to incomplete evidence. The API correctly documents this as
mutation detection rather than an atomic package-manager transaction snapshot.

#### Do not read `CONTENTS` when excluded

Resolved. `CONTENTS` remains a required regular-file commit marker, but its
payload is only read when `IncludeContents` is true.

#### Reject path-like package identities

Resolved. `ParsePackageID` explicitly rejects `.` and `..`, with adversarial
tests for both.

#### Define optional-file symlink policy

Resolved. Required and optional metadata use the same regular-file reader and
symlinks are rejected.

#### Add declared USE structure

Resolved. `DeclaredUse []UseDeclaration` exposes the flag name and
`UseDefaultEnabled`, `UseDefaultDisabled`, or `UseDefaultUnspecified`.

#### Stabilize release consumption

Resolved. Gentooling `v0.1.0` is tagged, Arise consumes the tag, and
`COMPATIBILITY.md` defines pre-1.0 minor-release changes, patch compatibility,
stable issue/error contracts, and deprecation expectations.

### Recheck findings: likely soon

#### Separate module stability from one large root package

There is currently one package containing paths, identities, issues, and VDB
scanning. That is appropriate for `v0.1.0`. As configuration, profiles,
repository access, atoms, and kernel requirement capture arrive, Gentooling
should expose cohesive packages or deliberately define one facade so Maize
does not inherit a large, tightly coupled root namespace.

#### Complete error-category coverage

The installed API preserves filesystem errors and now gives
`ErrConcurrentMutation` concrete semantics. Not-found behavior will need a
consistent contract when query APIs arrive. `Issue.Cause` is useful in-process
but should not itself become a serialized schema; snapshots need stable codes
and explicit fields.

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

## Gentooling v0.2.0 update

Gentooling `v0.2.0` at `9a1c308` adds the second public consumer slice:

- Explicit, root-aware active profile loading.
- Explicit repository roots for cross-repository parents.
- Root-to-leaf directories and layer models.
- Merged profile defaults, system set, package.provided, force/mask policy,
  and package-scoped USE policy.
- Source path and line provenance for package-scoped flag rules.
- Context cancellation, owned results, deterministic graph order, escape and
  cycle rejection, and symlink-safe policy reads.

Arise consumes `v0.2.0` at `4096614`. Maize consumes the same release through a
data-only adapter that translates profile layers into kernel-facing evidence;
it does not perform final USE evaluation.

This completes the active profile graph portion of requirement 4 and supplies
most profile inputs needed by requirement 3. The following consumer feedback
should inform the next API:

- `PolicySource` is attached to package-scoped rules, but `make.defaults`,
  `packages`, `package.provided`, global `use.force`/`use.mask`, and their
  stable variants do not retain source line numbers. Maize can cite the layer
  file but cannot give an exact operator location.
- Add/remove policy is encoded as string prefixes in several collections.
  Typed policy changes would prevent each consumer from reproducing removal
  semantics when explaining layers.
- A profile graph read is not yet part of a combined consistency snapshot.
  Final kernel generation still needs installed state, effective
  configuration, selections, and profile policy captured under one documented
  consistency contract.
- Effective `/etc/portage` configuration and final per-package USE evaluation
  remain absent. Maize must not treat raw profile force/mask/default layers as
  the final installed or prospective USE state.

The next useful Gentooling extraction remains effective configuration followed
by structured USE evaluation and world/system selection provenance.

## Gentooling v0.3.0 update

Gentooling `v0.3.0` at `bc89b03` adds explicit effective Portage configuration:

- `make.globals`, ordered profile defaults, user `make.conf`, and user
  `package.use`.
- An explicitly supplied command environment; `nil` never imports process
  state.
- Ordered profile, user, and command USE changes with source provenance.
- USE_EXPAND, USE_EXPAND_HIDDEN, and USE_EXPAND_IMPLICIT materialization.
- Owned results, context cancellation, malformed assignment rejection, and
  symlink-safe configuration policy.

Arise consumes `v0.3.0` at `7341314`. Maize consumes the release through a
data-only effective-configuration evidence adapter. This resolves the
root-aware configuration portion of requirement 2 and supplies the ordered
inputs for requirement 3.

Remaining consumer feedback:

- Final per-package USE evaluation is still absent by design. Maize must not
  independently implement atom matching, IUSE filtering/defaults, stable
  force/mask precedence, or overlapping package-rule reduction.
- Effective non-USE variables are returned as a map without their winning
  assignment provenance. Kernel explanations using `ARCH` or other variables
  can name the effective value but cannot cite its exact source.
- Profile-global policy still lacks source line provenance as noted in the
  v0.2.0 review.
- The effective configuration read is not yet combined atomically with
  installed inventory and selections.
- World selection provenance and the combined system snapshot remain pending.

The next highest-value extraction is final structured USE evaluation, followed
by world/system selections and the combined consistency contract.

## Gentooling v0.4.0 update

Gentooling `v0.4.0` adds the two package-policy contracts Maize had deliberately
left unimplemented:

- Public Gentoo atom and version parsing and matching, including versions,
  slots, repositories, and USE dependencies.
- Deterministic per-package USE evaluation over declared IUSE defaults,
  profile and user policy, command input, masks, forces, stable-only policy,
  and package-scoped rules, with ordered evidence.

Arise already consumes `v0.4.0` for shared package matching. Maize now uses the
same matcher for kernel-capability rules and exposes effective USE decisions
through a data-only adapter. Recorded installed USE remains distinct from
effective policy: the former says what an installed package was built with;
the latter supports prospective configuration analysis.

Resolved consumer requirements:

- Maize no longer needs an exact-CP limitation or a competing atom parser.
- IUSE filtering, defaults, package-rule matching, mask/force precedence, and
  stable-only policy evaluation remain Gentooling responsibilities.
- Failed reads and evaluations return no partial Maize evidence.

Remaining consumer feedback, ranked:

1. Needed now: world and system selections with provenance.
2. Needed now: a combined, consistent snapshot of installed inventory,
   effective configuration, profile policy, and selections.
3. Likely soon: keyword and visibility policy that can determine whether a
   prospective package context is stable. `EvaluateUse` correctly makes
   stability explicit, but Maize cannot infer it authoritatively yet.
4. Likely soon: winning-assignment provenance for effective non-USE variables.
5. Likely soon: source line provenance for profile-global policy.
6. Speculative: typed constants for `UseEvidence.Kind` and `UseEvidence.Layer`
   to make the consumer mapping compile-time checked.

The next highest-value extraction is selections and the combined consistency
contract. Keyword/visibility evaluation should follow before Maize recommends
prospective package-state changes.

## Gentooling v0.5.0 update

Gentooling `v0.5.0` resolves the remaining package-state aggregation boundary:

- Typed, deterministic world and effective profile system selections,
  distinguishing package atoms from named sets and retaining source lines.
- A combined snapshot of installed inventory, effective configuration/profile
  policy, and selections.
- Portage-compatible shared-lock observation for VDB and world state.
- Two agreeing complete observations, with persistent change reported as
  `ErrConcurrentMutation` instead of mixed evidence.

Arise consumes `v0.5.0` through the same snapshot contract. Maize now exposes a
data-only snapshot adapter that requires complete installed evidence, omits
CONTENTS payloads, accepts only an explicit command environment, and translates
selection provenance without weakening Gentooling's consistency guarantee.

Resolved consumer requirements:

- World and system selection provenance is available.
- Installed packages, effective policy, active profile, and selections can be
  reasoned about as one consistent input.
- Cooperating Portage and Arise writers are observed through their state locks.

Remaining consumer feedback, ranked:

1. Needed now: none for the originally requested package-state boundary.
2. Likely soon: keyword and visibility policy that authoritatively determines
   stability for prospective package contexts.
3. Likely soon: winning-assignment provenance for effective non-USE variables.
4. Likely soon: source line provenance for profile-global policy.
5. Speculative: typed constants for `UseEvidence.Kind` and `UseEvidence.Layer`.
6. Speculative: named-set expansion if Maize eventually needs the packages
   transitively selected by sets rather than the selection itself.

Maize can now proceed from fixture-fed package adapters to consistent real
Gentoo package-state evidence. Keyword/visibility evaluation is the next
important shared extraction before recommending prospective package changes.

## Gentooling v0.6.0 update

Gentooling `v0.6.0` adds authoritative prospective package visibility:

- Typed visible, package-masked, keyword-masked, and
  unsupported-architecture outcomes.
- Effective global and package-specific keyword policy with ordered changes
  and source provenance.
- Repository, active-profile, and user mask/unmask stacks with removal
  semantics and mask rationale.
- An authoritative stability result for downstream stable-only USE policy.

Arise consumes `v0.6.0`. Maize now evaluates visibility and USE policy from one
effective configuration read. The visibility result supplies the `Stable`
input to Gentooling's USE evaluator, removing the temporary caller-supplied
stability assumption from prospective package analysis. Ordinary package or
keyword rejection remains explainable evidence rather than an operational
error.

Resolved consumer requirements:

- Maize can distinguish a stable candidate from a testing candidate using
  effective Gentoo policy.
- Stable-only USE force/mask policy can be applied without local keyword
  parsing or inference.
- Package mask reasons and keyword policy inputs can appear in operator-facing
  explanations.

Remaining consumer feedback, ranked:

1. Needed now: none for package state, selection, visibility, and USE policy.
2. Likely soon: winning-assignment provenance for effective non-USE variables.
3. Likely soon: source line provenance for profile-global policy.
4. Likely soon: a snapshot-bound prospective evaluation API if Maize must
   guarantee that repository candidate policy is evaluated against the exact
   same observation as installed state and selections.
5. Speculative: typed constants for visibility and USE evidence kinds/layers.
6. Speculative: named-set expansion.

The shared Gentooling boundary is sufficient for Maize's first real
package-aware kernel recommendations. Further extraction should now be driven
by concrete adapter friction rather than preemptive Portage reimplementation.

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
