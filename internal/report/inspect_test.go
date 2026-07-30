package report_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/airencracken/maize/internal/app"
	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/kernel"
	"github.com/airencracken/maize/internal/recommend"
	"github.com/airencracken/maize/internal/report"
)

func TestInspectionJSONEmitsVersionedDeterministicContract(t *testing.T) {
	t.Parallel()

	inspection := reportFixture()
	var first bytes.Buffer
	if err := report.InspectionJSON(&first, inspection); err != nil {
		t.Fatal(err)
	}
	var second bytes.Buffer
	if err := report.InspectionJSON(&second, inspection); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatalf("JSON changed:\n%s\n%s", first.String(), second.String())
	}
	var document struct {
		Schema           string `json:"schema"`
		InstalledCount   int    `json:"installed_count"`
		WorldSelections  []any  `json:"world_selections"`
		SystemSelections []any  `json:"system_selections"`
		Recommendations  []struct {
			Capability  string  `json:"capability"`
			Disposition string  `json:"disposition"`
			Symbol      string  `json:"symbol"`
			Current     *string `json:"current"`
			Desired     string  `json:"desired"`
			Action      string  `json:"action"`
			Detail      string  `json:"detail"`
			Evidence    []struct {
				Kind       string `json:"kind"`
				Source     string `json:"source"`
				Detail     string `json:"detail"`
				Confidence string `json:"confidence"`
			} `json:"evidence"`
		} `json:"recommendations"`
	}
	decoder := json.NewDecoder(strings.NewReader(first.String()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode contract: %v\n%s", err, first.String())
	}
	if document.Schema != app.InspectSchema || document.InstalledCount != 1 ||
		len(document.Recommendations) != 1 ||
		document.Recommendations[0].Symbol != "CONFIG_SECCOMP_FILTER" ||
		document.Recommendations[0].Current == nil ||
		*document.Recommendations[0].Current != "n" ||
		document.Recommendations[0].Desired != "y" ||
		document.Recommendations[0].Action != "enable" {
		t.Fatalf("document = %#v", document)
	}
}

func TestInspectionTextExplainsMaterialDecision(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := report.InspectionText(&output, reportFixture()); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Installed packages: 1",
		"CONFIG_SECCOMP_FILTER: n -> y (enable, required)",
		"because app-containers/docker-28.3.2[seccomp]",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("report missing %q:\n%s", expected, output.String())
		}
	}
}

func TestInspectionReportsPropagateWriterFailures(t *testing.T) {
	t.Parallel()

	writer := failingWriter{}
	if err := report.InspectionText(writer, reportFixture()); err == nil {
		t.Fatal("text writer error was ignored")
	}
	if err := report.InspectionJSON(writer, reportFixture()); err == nil {
		t.Fatal("JSON writer error was ignored")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("injected write failure")
}

func reportFixture() app.Inspection {
	symbol, _ := kernel.ParseSymbol("SECCOMP_FILTER")
	current := kernel.No()
	return app.Inspection{
		Schema: app.InspectSchema, InstalledCount: 1,
		Recommendations: []recommend.Recommendation{{
			Capability: "security.seccomp-filter", Disposition: domain.Required,
			Symbol: symbol, Current: &current, Desired: kernel.Yes(),
			Action: recommend.ActionEnable, Detail: "provide seccomp filtering",
			Evidence: []domain.Evidence{{
				Kind: domain.SourceUseFlag, Source: "app-containers/docker-28.3.2[seccomp]",
				Detail: "Docker was built with seccomp support", Confidence: domain.Certain,
			}},
		}},
	}
}
