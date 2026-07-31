package terminal

import (
	"fmt"
	"io"
	"os"
)

type ColorMode string

const (
	ColorAuto   ColorMode = "auto"
	ColorAlways ColorMode = "always"
	ColorNever  ColorMode = "never"
)

func ParseColorMode(value string) (ColorMode, error) {
	mode := ColorMode(value)
	switch mode {
	case ColorAuto, ColorAlways, ColorNever:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid color mode %q; want auto, always, or never", value)
	}
}

type Style struct {
	enabled bool
}

func StyleForWriter(mode ColorMode, writer io.Writer) Style {
	switch mode {
	case ColorAlways:
		return Style{enabled: true}
	case ColorNever:
		return Style{}
	}
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return Style{}
	}
	file, ok := writer.(*os.File)
	if !ok {
		return Style{}
	}
	info, err := file.Stat()
	return Style{enabled: err == nil && info.Mode()&os.ModeCharDevice != 0}
}

func (s Style) Enabled() bool                  { return s.enabled }
func (s Style) Bold(value string) string       { return s.wrap(value, "\x1b[1m") }
func (s Style) Red(value string) string        { return s.wrap(value, "\x1b[31m") }
func (s Style) Green(value string) string      { return s.wrap(value, "\x1b[32m") }
func (s Style) Yellow(value string) string     { return s.wrap(value, "\x1b[33m") }
func (s Style) Cyan(value string) string       { return s.wrap(value, "\x1b[36m") }
func (s Style) Magenta(value string) string    { return s.wrap(value, "\x1b[35m") }
func (s Style) BoldRed(value string) string    { return s.wrap(value, "\x1b[1m\x1b[31m") }
func (s Style) BoldGreen(value string) string  { return s.wrap(value, "\x1b[1m\x1b[32m") }
func (s Style) BoldYellow(value string) string { return s.wrap(value, "\x1b[1m\x1b[33m") }
func (s Style) BoldCyan(value string) string   { return s.wrap(value, "\x1b[1m\x1b[36m") }

func (s Style) wrap(value, prefix string) string {
	if !s.enabled || value == "" {
		return value
	}
	return prefix + value + "\x1b[0m"
}
