# Maize

Maize generates and migrates optimized Linux kernel configurations for Gentoo.
It combines current and previously observed hardware, Portage state, installed
package USE flags, operator profiles, an existing kernel configuration, and the
target kernel's Kconfig model.

Maize is currently an architectural sketch. See [docs/plan.md](docs/plan.md) for
the product plan and [docs/architecture.md](docs/architecture.md) for the
initial component boundaries.

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
