package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `maize generates and migrates optimized Gentoo kernel configurations

Usage:
  maize inspect
  maize generate
  maize migrate
  maize check
  maize impact
  maize observe
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "inspect", "generate", "migrate", "check", "impact", "observe":
		fmt.Fprintf(stderr, "maize %s is not implemented yet\n", args[0])
		return 2
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
