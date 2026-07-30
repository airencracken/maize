# Maize

Maize generates and migrates optimized Linux kernel configurations for Gentoo.
It combines current and previously observed hardware, Portage state, installed
package USE flags, operator profiles, an existing kernel configuration, and the
target kernel's Kconfig model.

Maize is under active foundation development. It currently provides normalized
kernel and evidence models, strict Linux `.config` parsing, an explanatory
Kconfig parser, semantic migration classification, hardware inventory values,
and strict installed-package evidence through Gentooling. See
[docs/plan.md](docs/plan.md) for the product plan and
[docs/architecture.md](docs/architecture.md) for component boundaries.

See [docs/prior-art.md](docs/prior-art.md) for acknowledgment of earlier work,
including nichoski/kergen.

## Intended commands

```text
maize inspect
maize generate
maize migrate --target /usr/src/linux
maize check
maize impact --config ./proposed.config
```

The intended output is a validated `.config` plus an audit report explaining
each material decision, its evidence, its confidence, and any unresolved
operator choices.

## Development

```text
go test ./...
go test -race ./...
go vet ./...
```

## License

Maize is licensed under the GNU General Public License version 3 only
(`GPL-3.0-only`). See [LICENSE](LICENSE).
