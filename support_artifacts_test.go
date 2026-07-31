package maize_test

import (
	"os"
	"strings"
	"testing"
)

func TestOperatorSupportArtifactsCoverEveryCommand(t *testing.T) {
	t.Parallel()

	paths := []string{
		"completions/maize.bash",
		"docs/maize.1",
		"docs/maize.texi",
	}
	contents := make(map[string]string, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		contents[path] = string(data)
	}
	for _, command := range []string{"inspect", "generate", "migrate", "check", "impact", "observe"} {
		for path, content := range contents {
			if !strings.Contains(content, command) {
				t.Errorf("%s does not document command %q", path, command)
			}
		}
	}
}

func TestDocumentationRecordsColorAndCheckExitContract(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"docs/maize.1", "docs/maize.texi"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		for _, required := range []string{"NO_COLOR", "auto", "always", "never", "indeterminate"} {
			if !strings.Contains(content, required) {
				t.Errorf("%s does not contain %q", path, required)
			}
		}
	}
}
