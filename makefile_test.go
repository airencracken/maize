package maize_test

import (
	"os"
	"strings"
	"testing"
)

func TestMakefileExposesDocumentedDevelopmentContract(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	makefile := string(data)
	for _, target := range []string{
		"all:", "build:", "check:", "fmt:", "fmt-check:", "help:",
		"install:", "mod-check:", "test:", "test-race:", "vet:",
	} {
		if !strings.Contains(makefile, "\n"+target) {
			t.Errorf("Makefile does not declare %q", strings.TrimSuffix(target, ":"))
		}
	}
	if !strings.Contains(makefile, "check: fmt-check mod-check vet test test-race") {
		t.Error("check target does not run the complete validation contract")
	}
	if !strings.Contains(makefile, `-buildvcs=false -o "$(BIN_DIR)/maize" ./cmd/maize`) {
		t.Error("build target does not compile the maize command")
	}
}

func TestMakefileDoesNotUseFailFastShellModes(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"set -e", "set -o pipefail", "set -u"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("Makefile contains forbidden shell mode %q", forbidden)
		}
	}
}
