# Maize

Maize generates and migrates optimized Linux kernel configurations for Gentoo.
It combines current and previously observed hardware, Portage state, installed
package USE flags, operator profiles, an existing kernel configuration, and the
target kernel's Kconfig model.

Maize is under active development. Its first executable pipeline is
`maize inspect`: it reads a consistent Gentoo system snapshot, walks current
sysfs hardware, and discovers or accepts an existing Linux `.config`. It
applies reviewed installed-package and USE rules and reports the kernel symbols
that are already satisfied or should change. Structured static kernel
requirements from installed packages' ebuild/eclass sources are included;
shell-dependent policy remains explicit and blocks generation. It also
provides normalized kernel and evidence models, an explanatory Kconfig parser,
semantic migration classification, and hardware inventory values. See
[docs/plan.md](docs/plan.md) for the product plan and
[docs/architecture.md](docs/architecture.md) for component boundaries.

See [docs/prior-art.md](docs/prior-art.md) for acknowledgment of earlier work,
including nichoski/kergen.

## Commands

```text
maize inspect
maize inspect --config /usr/src/linux/.config
maize inspect --format json
maize generate --kernel-tree /usr/src/linux --output ./candidate.config
maize migrate
maize migrate --old-kconfig OLD --new-kconfig NEW --old-config OLD --new-config NEW
maize check --snapshot-consistency stabilized
maize impact --config ./proposed.config
maize observe --output ./hardware.json
```

All declared commands have an executable first implementation. `inspect`,
`check`, `impact`, and `migrate` are read-only. `generate` and `observe` require
explicit output paths and write atomically.

Text reports use terminal-aware color inspired by Arise's semantic palette.
Use `--color auto|always|never`; automatic mode also honors `NO_COLOR`.
Unresolved package policy is summarized by default and expanded with
`--verbose`. JSON output is never colorized.

`generate` requires an explicit target kernel source tree. It runs that tree's
`olddefconfig` in an isolated output directory, verifies that required Maize
decisions survived Kconfig dependency resolution, and atomically writes only
the validated result. Hardware-to-Kconfig translation and configuration
minimization remain incomplete, so generated configurations are not yet
claimed to be optimal.

With no artifact overrides, `migrate` loads the running kernel configuration,
discovers versioned source trees below `/usr/src`, selects the newest release
using kernel-version ordering, and resolves the running configuration through
that target tree. The report always identifies the running release, config
source, target release, and selected source tree.

Consistent snapshots observe Portage state locks by default. Run `inspect` as a
user that can read those lock files, or explicitly request two-pass lockless
stabilization with `--snapshot-consistency stabilized`. Gentooling discovers
root-aware repository configuration; repeatable `--repository NAME=PATH`
options remain available as explicit overrides.

Without `--config`, inspection prefers `/proc/config.gz`, then the running
release's `/boot/config-*`, then `/usr/src/linux/.config`. Alternate roots may
provide explicit virtual filesystem locations:

```text
maize inspect --root /mnt/gentoo --sysfs /mnt/gentoo/sys --procfs /mnt/gentoo/proc
```

## Development

```text
make
```

The default target runs formatting and module consistency checks, `go vet`,
the normal and race-enabled test suites, and then builds `bin/maize`. Run
`make help` for focused targets such as `make test`, `make build`, and
`make install`. `make docs` builds the Info manual, while `make install-docs`
installs Bash completion, the `maize(1)` man page, and the Info manual beneath
`PREFIX` (default `/usr/local`).

## License

Maize is licensed under the GNU General Public License version 3 only
(`GPL-3.0-only`). See [LICENSE](LICENSE).
