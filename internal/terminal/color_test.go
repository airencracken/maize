package terminal_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/terminal"
)

func TestParseColorModeAcceptsContractAndRejectsAdversarialValues(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"auto", "always", "never"} {
		if mode, err := terminal.ParseColorMode(value); err != nil || string(mode) != value {
			t.Fatalf("ParseColorMode(%q) = %q, %v", value, mode, err)
		}
	}
	for _, value := range []string{"", "yes", "AUTO", "\x1b[31m"} {
		if _, err := terminal.ParseColorMode(value); err == nil {
			t.Errorf("ParseColorMode(%q) accepted invalid input", value)
		}
	}
}

func TestForcedStylesUseStableAriseInspiredPalette(t *testing.T) {
	t.Parallel()

	style := terminal.StyleForWriter(terminal.ColorAlways, &bytes.Buffer{})
	tests := map[string]string{
		style.Red("bad"):          "\x1b[31mbad\x1b[0m",
		style.Green("good"):       "\x1b[32mgood\x1b[0m",
		style.Yellow("warn"):      "\x1b[33mwarn\x1b[0m",
		style.Cyan("info"):        "\x1b[36minfo\x1b[0m",
		style.Magenta("package"):  "\x1b[35mpackage\x1b[0m",
		style.BoldCyan("heading"): "\x1b[1m\x1b[36mheading\x1b[0m",
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("style = %q, want %q", got, want)
		}
	}
}

func TestNeverAndAutoForNonTerminalProducePlainOwnedText(t *testing.T) {
	t.Parallel()

	for _, mode := range []terminal.ColorMode{terminal.ColorNever, terminal.ColorAuto} {
		style := terminal.StyleForWriter(mode, &bytes.Buffer{})
		if style.Enabled() || style.BoldRed("required") != "required" {
			t.Fatalf("mode %q enabled non-terminal color", mode)
		}
	}
}

func TestAutoHonorsNoColor(t *testing.T) {
	previous, existed := os.LookupEnv("NO_COLOR")
	if err := os.Setenv("NO_COLOR", "1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv("NO_COLOR", previous)
		} else {
			_ = os.Unsetenv("NO_COLOR")
		}
	})
	style := terminal.StyleForWriter(terminal.ColorAuto, os.Stdout)
	if style.Enabled() || strings.Contains(style.Green("ok"), "\x1b[") {
		t.Fatal("NO_COLOR did not disable automatic color")
	}
}
