package kernel

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

type SymbolType string

const (
	TypeUnknown  SymbolType = ""
	TypeBool     SymbolType = "bool"
	TypeTristate SymbolType = "tristate"
	TypeString   SymbolType = "string"
	TypeInteger  SymbolType = "int"
	TypeHex      SymbolType = "hex"
)

type Definition struct {
	Symbol    Symbol
	Type      SymbolType
	Prompt    string
	DependsOn []string
	Defaults  []string
	Selects   []string
	Implies   []string
	Help      string
	Location  Location
}

type Catalog struct {
	definitions map[Symbol]Definition
}

func NewCatalog(definitions ...Definition) (Catalog, error) {
	catalog := Catalog{definitions: make(map[Symbol]Definition)}
	for _, definition := range definitions {
		if previous, exists := catalog.definitions[definition.Symbol]; exists {
			return Catalog{}, fmt.Errorf(
				"duplicate %s at %s:%d and %s:%d",
				definition.Symbol.String(), previous.Location.Path, previous.Location.Line,
				definition.Location.Path, definition.Location.Line,
			)
		}
		catalog.definitions[definition.Symbol] = cloneDefinition(definition)
	}
	return catalog, nil
}

func ParseKconfig(path string, reader io.Reader) (Catalog, error) {
	var definitions []Definition
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var current *Definition
	lineNumber := 0
	inHelp := false
	var helpIndent int
	var help []string

	flushHelp := func() {
		if current != nil && len(help) != 0 {
			current.Help = strings.TrimSpace(strings.Join(help, "\n"))
		}
		help = nil
		inHelp = false
	}
	flushDefinition := func() {
		flushHelp()
		if current != nil {
			definitions = append(definitions, *current)
			current = nil
		}
	}

	for scanner.Scan() {
		lineNumber++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))

		if inHelp {
			if trimmed == "" {
				help = append(help, "")
				continue
			}
			if indent > helpIndent {
				help = append(help, strings.TrimSpace(raw))
				continue
			}
			flushHelp()
		}

		fields := strings.Fields(trimmed)
		if len(fields) >= 2 && (fields[0] == "config" || fields[0] == "menuconfig") {
			flushDefinition()
			symbol, err := ParseSymbol(fields[1])
			if err != nil {
				return Catalog{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
			current = &Definition{Symbol: symbol, Location: Location{Path: path, Line: lineNumber}}
			continue
		}
		if current == nil || len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "bool", "tristate", "string", "int", "hex":
			current.Type = SymbolType(fields[0])
			current.Prompt = quotedArgument(strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0])))
		case "prompt":
			current.Prompt = quotedArgument(strings.TrimSpace(strings.TrimPrefix(trimmed, "prompt")))
		case "depends":
			if len(fields) >= 3 && fields[1] == "on" {
				current.DependsOn = append(current.DependsOn, strings.Join(fields[2:], " "))
			}
		case "default":
			current.Defaults = append(current.Defaults, strings.TrimSpace(strings.TrimPrefix(trimmed, "default")))
		case "select":
			current.Selects = append(current.Selects, strings.TrimSpace(strings.TrimPrefix(trimmed, "select")))
		case "imply":
			current.Implies = append(current.Implies, strings.TrimSpace(strings.TrimPrefix(trimmed, "imply")))
		case "help", "---help---":
			inHelp = true
			helpIndent = indent
		}
	}
	if err := scanner.Err(); err != nil {
		return Catalog{}, fmt.Errorf("%s: read Kconfig: %w", path, err)
	}
	flushDefinition()
	return NewCatalog(definitions...)
}

func quotedArgument(value string) string {
	if !strings.HasPrefix(value, "\"") {
		return ""
	}
	value = strings.TrimPrefix(value, "\"")
	if end := strings.IndexByte(value, '"'); end >= 0 {
		return value[:end]
	}
	return value
}

func (c Catalog) Get(symbol Symbol) (Definition, bool) {
	definition, found := c.definitions[symbol]
	return cloneDefinition(definition), found
}

func (c Catalog) Definitions() []Definition {
	result := make([]Definition, 0, len(c.definitions))
	for _, definition := range c.definitions {
		result = append(result, cloneDefinition(definition))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Symbol < result[j].Symbol })
	return result
}

func cloneDefinition(definition Definition) Definition {
	definition.DependsOn = append([]string(nil), definition.DependsOn...)
	definition.Defaults = append([]string(nil), definition.Defaults...)
	definition.Selects = append([]string(nil), definition.Selects...)
	definition.Implies = append([]string(nil), definition.Implies...)
	return definition
}
