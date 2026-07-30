package domain_test

import (
	"testing"

	"github.com/airencracken/maize/internal/domain"
)

func TestExplanationValidation(t *testing.T) {
	t.Parallel()

	explanation := domain.Explanation{
		Summary:    "CONFIG_EXT4_FS is required for the root filesystem",
		Confidence: domain.Certain,
		Provenance: []domain.Provenance{{
			Kind: domain.SourceFilesystem, Source: "/", Detail: "root is mounted as ext4",
		}},
	}
	if err := explanation.Validate(); err != nil {
		t.Fatalf("valid explanation rejected: %v", err)
	}
	explanation.Provenance[0].Source = ""
	if err := explanation.Validate(); err == nil {
		t.Fatal("invalid provenance accepted")
	}
}
