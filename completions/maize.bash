# Bash completion for maize.

_maize()
{
	local cur prev command
	COMPREPLY=()
	cur=${COMP_WORDS[COMP_CWORD]}
	prev=
	if (( COMP_CWORD > 0 )); then
		prev=${COMP_WORDS[COMP_CWORD-1]}
	fi
	command=
	if (( COMP_CWORD > 1 )); then
		command=${COMP_WORDS[1]}
	fi

	if (( COMP_CWORD == 1 )); then
		COMPREPLY=( $(compgen -W "inspect generate migrate check impact observe help" -- "$cur") )
		return
	fi

	case "$prev" in
		--color)
			COMPREPLY=( $(compgen -W "auto always never" -- "$cur") )
			return
			;;
		--format)
			COMPREPLY=( $(compgen -W "text json" -- "$cur") )
			return
			;;
		--snapshot-consistency)
			COMPREPLY=( $(compgen -W "locked stabilized" -- "$cur") )
			return
			;;
		--kernel-tree|--root|--sysfs|--procfs)
			COMPREPLY=( $(compgen -d -- "$cur") )
			return
			;;
		--config|--old-kconfig|--new-kconfig|--old-config|--new-config|--output)
			COMPREPLY=( $(compgen -f -- "$cur") )
			return
			;;
	esac

	case "$cur" in
		--color=*)
			local value=${cur#--color=}
			COMPREPLY=( $(compgen -W "auto always never" -- "$value") )
			COMPREPLY=( "${COMPREPLY[@]/#/--color=}" )
			return
			;;
		--format=*)
			local value=${cur#--format=}
			COMPREPLY=( $(compgen -W "text json" -- "$value") )
			COMPREPLY=( "${COMPREPLY[@]/#/--format=}" )
			return
			;;
	esac

	local common="--color --config --format --help --procfs --repository --root --snapshot-consistency --sysfs --verbose"
	case "$command" in
		inspect|check)
			COMPREPLY=( $(compgen -W "$common" -- "$cur") )
			;;
		generate)
			COMPREPLY=( $(compgen -W "$common --experimental-best-guess --experimental-minimize --kernel-tree --output" -- "$cur") )
			;;
		impact)
			COMPREPLY=( $(compgen -W "$common" -- "$cur") )
			;;
		migrate)
			COMPREPLY=( $(compgen -W "--color --format --help --new-config --new-kconfig --old-config --old-kconfig --procfs --root --verbose" -- "$cur") )
			;;
		observe)
			COMPREPLY=( $(compgen -W "--color --help --output --root --sysfs" -- "$cur") )
			;;
	esac
}

complete -F _maize maize
