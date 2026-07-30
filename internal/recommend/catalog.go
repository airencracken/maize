package recommend

import (
	"github.com/airencracken/maize/internal/domain"
	maizegentoo "github.com/airencracken/maize/internal/gentooling"
	"github.com/airencracken/maize/internal/kernel"
)

// PackageRules is the initial reviewed Gentoo package-to-capability catalog.
// Presence-only rules are recommendations; recorded feature USE is required.
func PackageRules() []maizegentoo.Rule {
	return []maizegentoo.Rule{
		{Atom: "app-containers/docker", Capability: "containers", Disposition: domain.Recommended, Confidence: domain.High, Detail: "Docker is installed"},
		{Atom: "app-containers/docker", UseFlag: "apparmor", Capability: "security.apparmor", Disposition: domain.Required, Confidence: domain.Certain, Detail: "Docker was built with AppArmor support"},
		{Atom: "app-containers/docker", UseFlag: "seccomp", Capability: "security.seccomp-filter", Disposition: domain.Required, Confidence: domain.Certain, Detail: "Docker was built with seccomp support"},
		{Atom: "net-firewall/nftables", Capability: "network.nftables", Disposition: domain.Recommended, Confidence: domain.High, Detail: "nftables is installed"},
		{Atom: "net-vpn/wireguard-tools", Capability: "network.wireguard", Disposition: domain.Recommended, Confidence: domain.High, Detail: "WireGuard tools are installed"},
		{Atom: "sys-fs/btrfs-progs", Capability: "filesystem.btrfs", Disposition: domain.Recommended, Confidence: domain.Medium, Detail: "Btrfs userspace tools are installed"},
		{Atom: "sys-fs/cryptsetup", Capability: "storage.dm-crypt", Disposition: domain.Recommended, Confidence: domain.High, Detail: "cryptsetup is installed"},
	}
}

func KernelBindings() []Binding {
	return []Binding{
		{Capability: "containers", Symbol: mustSymbol("CONFIG_CGROUPS"), State: kernel.Yes(), Detail: "control groups are a container runtime foundation"},
		{Capability: "containers", Symbol: mustSymbol("CONFIG_NAMESPACES"), State: kernel.Yes(), Detail: "namespaces are a container runtime foundation"},
		{Capability: "filesystem.btrfs", Symbol: mustSymbol("CONFIG_BTRFS_FS"), State: kernel.Module(), Detail: "provide the Btrfs filesystem driver"},
		{Capability: "network.nftables", Symbol: mustSymbol("CONFIG_NF_TABLES"), State: kernel.Module(), Detail: "provide the nftables packet classification framework"},
		{Capability: "network.wireguard", Symbol: mustSymbol("CONFIG_WIREGUARD"), State: kernel.Module(), Detail: "provide the WireGuard tunnel driver"},
		{Capability: "security.apparmor", Symbol: mustSymbol("CONFIG_SECURITY_APPARMOR"), State: kernel.Yes(), Detail: "provide AppArmor security policy support"},
		{Capability: "security.seccomp-filter", Symbol: mustSymbol("CONFIG_SECCOMP"), State: kernel.Yes(), Detail: "provide secure computing mode"},
		{Capability: "security.seccomp-filter", Symbol: mustSymbol("CONFIG_SECCOMP_FILTER"), State: kernel.Yes(), Detail: "provide seccomp BPF filtering"},
		{Capability: "storage.dm-crypt", Symbol: mustSymbol("CONFIG_BLK_DEV_DM"), State: kernel.Module(), Detail: "provide the device-mapper core"},
		{Capability: "storage.dm-crypt", Symbol: mustSymbol("CONFIG_DM_CRYPT"), State: kernel.Module(), Detail: "provide device-mapper encryption"},
	}
}

func mustSymbol(value string) kernel.Symbol {
	symbol, err := kernel.ParseSymbol(value)
	if err != nil {
		panic(err)
	}
	return symbol
}
