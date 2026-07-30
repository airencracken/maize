// Package kernel models Linux Kconfig symbols and configuration migration.
package kernel

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Symbol string

func ParseSymbol(value string) (Symbol, error) {
	value = strings.TrimPrefix(value, "CONFIG_")
	if value == "" {
		return "", errors.New("kernel symbol is empty")
	}
	for _, r := range value {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid kernel symbol %q", value)
	}
	return Symbol(value), nil
}

func (s Symbol) String() string {
	return "CONFIG_" + string(s)
}

type StateKind string

const (
	StateNo      StateKind = "no"
	StateModule  StateKind = "module"
	StateYes     StateKind = "yes"
	StateString  StateKind = "string"
	StateInteger StateKind = "integer"
	StateHex     StateKind = "hex"
)

type State struct {
	Kind  StateKind
	Value string
}

func No() State     { return State{Kind: StateNo} }
func Module() State { return State{Kind: StateModule} }
func Yes() State    { return State{Kind: StateYes} }

func ParseState(value string) (State, error) {
	switch value {
	case "n":
		return No(), nil
	case "m":
		return Module(), nil
	case "y":
		return Yes(), nil
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		decoded, err := strconv.Unquote(value)
		if err != nil {
			return State{}, fmt.Errorf("invalid quoted kernel value: %w", err)
		}
		return State{Kind: StateString, Value: decoded}, nil
	}
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		if _, err := strconv.ParseUint(value[2:], 16, 64); err != nil {
			return State{}, fmt.Errorf("invalid hexadecimal kernel value %q", value)
		}
		return State{Kind: StateHex, Value: strings.ToLower(value)}, nil
	}
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return State{Kind: StateInteger, Value: value}, nil
	}
	return State{}, fmt.Errorf("unsupported kernel value %q", value)
}

func (s State) ConfigValue() string {
	switch s.Kind {
	case StateNo:
		return "n"
	case StateModule:
		return "m"
	case StateYes:
		return "y"
	case StateString:
		return strconv.Quote(s.Value)
	default:
		return s.Value
	}
}

type Location struct {
	Path string
	Line int
}

type Entry struct {
	Symbol   Symbol
	State    State
	Location Location
}
