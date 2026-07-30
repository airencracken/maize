# Prior art

Maize acknowledges
[nichoski/kergen](https://github.com/nichoski/kergen) as important prior art
for automated Linux kernel configuration.

Kergen demonstrated several core ideas that inform this problem space:

- Discover hardware through Linux system interfaces.
- Map PCI, USB, and SCSI devices and mounted filesystems to kernel options.
- Parse Kconfig dependencies and select a satisfiable dependency set.
- Carry an existing configuration forward with the kernel's `olddefconfig`
  workflow.
- Treat automation as an aid to operator configuration rather than a reason to
  conceal uncertain choices.

Maize shares the goal of saving operators from repetitive kernel configuration
work, but has a different scope:

- It is written in Go and is specifically for Gentoo.
- It treats Portage state, profiles, USE policy, and installed packages as
  first-class evidence through Gentooling.
- It focuses on semantic migration explanations between kernel versions.
- It retains provenance and confidence for hardware, package, profile, and
  operator evidence.
- It optimizes against explicit machine and workload profiles.
- It delegates final configuration semantics to the target kernel's own
  Kconfig implementation.

No kergen source code is incorporated into Maize.
