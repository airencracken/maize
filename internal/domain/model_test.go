package domain_test

import (
	"strings"
	"testing"

	"github.com/marcus/maize/internal/domain"
)

func TestRequirementValidate(t *testing.T) {
	t.Parallel()

	valid := domain.Requirement{
		Capability:  "root.ext4",
		Disposition: domain.Required,
		Evidence: domain.Evidence{
			Kind:       domain.SourceFilesystem,
			Source:     "/",
			Detail:     "root filesystem is ext4",
			Confidence: domain.Certain,
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid requirement rejected: %v", err)
	}
}

func TestRequirementValidateRejectsAdversarialCapabilities(t *testing.T) {
	t.Parallel()

	values := []string{
		"",
		" CONFIG_KVM",
		"CONFIG_KVM",
		"../../etc/passwd",
		"containers\nprohibited",
		"-containers",
		strings.Repeat("a", 32) + "\x00",
	}
	for _, value := range values {
		value := value
		t.Run(value, func(t *testing.T) {
			requirement := domain.Requirement{
				Capability:  value,
				Disposition: domain.Required,
				Evidence: domain.Evidence{
					Kind:       domain.SourceProfile,
					Source:     "test",
					Detail:     "test input",
					Confidence: domain.Certain,
				},
			}
			if err := requirement.Validate(); err == nil {
				t.Fatalf("invalid capability %q accepted", value)
			}
		})
	}
}

func TestEvidenceValidateRejectsUnknownEnumValues(t *testing.T) {
	t.Parallel()

	evidence := domain.Evidence{
		Kind:       domain.SourceKind("guess"),
		Source:     "test",
		Detail:     "test input",
		Confidence: domain.Confidence("maybe"),
	}
	if err := evidence.Validate(); err == nil {
		t.Fatal("unknown enum values accepted")
	}
}
