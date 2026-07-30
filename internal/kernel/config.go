package kernel

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Config struct {
	entries map[Symbol]Entry
}

func ParseConfig(path string, reader io.Reader) (Config, error) {
	config := Config{entries: make(map[Symbol]Entry)}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "# Automatically generated") ||
			strings.HasPrefix(line, "# Linux/") || line == "#" {
			continue
		}

		var name, raw string
		if strings.HasPrefix(line, "# CONFIG_") && strings.HasSuffix(line, " is not set") {
			name = strings.TrimSuffix(strings.TrimPrefix(line, "# "), " is not set")
			raw = "n"
		} else if strings.HasPrefix(line, "CONFIG_") {
			var found bool
			name, raw, found = strings.Cut(line, "=")
			if !found {
				return Config{}, fmt.Errorf("%s:%d: configuration assignment has no value", path, lineNumber)
			}
		} else if strings.HasPrefix(line, "#") {
			continue
		} else {
			return Config{}, fmt.Errorf("%s:%d: unrecognized configuration line", path, lineNumber)
		}

		symbol, err := ParseSymbol(name)
		if err != nil {
			return Config{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		state, err := ParseState(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s:%d: %s: %w", path, lineNumber, symbol.String(), err)
		}
		if previous, exists := config.entries[symbol]; exists {
			return Config{}, fmt.Errorf(
				"%s:%d: duplicate %s previously set at line %d",
				path, lineNumber, symbol.String(), previous.Location.Line,
			)
		}
		config.entries[symbol] = Entry{
			Symbol: symbol, State: state, Location: Location{Path: path, Line: lineNumber},
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("%s: read configuration: %w", path, err)
	}
	return config, nil
}

func (c Config) Get(symbol Symbol) (Entry, bool) {
	entry, found := c.entries[symbol]
	return entry, found
}

func (c Config) Entries() []Entry {
	result := make([]Entry, 0, len(c.entries))
	for _, entry := range c.entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Symbol < result[j].Symbol })
	return result
}
