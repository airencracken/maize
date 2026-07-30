package resolve_test

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/airencracken/maize/internal/domain"
	"github.com/airencracken/maize/internal/resolve"
)

func TestRequirementsCombinesEvidenceAndUsesStrongestDisposition(t *testing.T) {
	t.Parallel()

	requirements := []domain.Requirement{
		requirement("containers", domain.Recommended, domain.SourceProfile, "workstation"),
		requirement("containers", domain.Required, domain.SourceUseFlag, "app-containers/docker[seccomp]"),
	}

	decisions, err := resolve.Requirements(requirements)
	if err != nil {
		t.Fatalf("resolve requirements: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(decisions))
	}
	if decisions[0].Disposition != domain.Required {
		t.Fatalf("got disposition %q, want %q", decisions[0].Disposition, domain.Required)
	}
	if len(decisions[0].Evidence) != 2 {
		t.Fatalf("got %d evidence records, want 2", len(decisions[0].Evidence))
	}
}

func TestRequirementsReportsConflictAtomically(t *testing.T) {
	t.Parallel()

	requirements := []domain.Requirement{
		requirement("containers", domain.Required, domain.SourcePackage, "app-containers/docker"),
		requirement("containers", domain.Prohibited, domain.SourceProfile, "hardened"),
		requirement("root.ext4", domain.Required, domain.SourceFilesystem, "/"),
	}

	decisions, err := resolve.Requirements(requirements)
	if !errors.Is(err, resolve.ErrConflict) {
		t.Fatalf("got error %v, want ErrConflict", err)
	}
	if decisions != nil {
		t.Fatalf("conflicted resolution returned partial decisions: %#v", decisions)
	}

	var conflictError *resolve.ConflictError
	if !errors.As(err, &conflictError) {
		t.Fatalf("error type %T, want *resolve.ConflictError", err)
	}
	if len(conflictError.Conflicts) != 1 || conflictError.Conflicts[0].Capability != "containers" {
		t.Fatalf("unexpected conflicts: %#v", conflictError.Conflicts)
	}
}

func TestRequirementsValidationFailureIsAtomic(t *testing.T) {
	t.Parallel()

	requirements := []domain.Requirement{
		requirement("root.ext4", domain.Required, domain.SourceFilesystem, "/"),
		requirement("../../invalid", domain.Required, domain.SourceProfile, "hostile"),
	}
	decisions, err := resolve.Requirements(requirements)
	if err == nil {
		t.Fatal("invalid requirement accepted")
	}
	if decisions != nil {
		t.Fatalf("validation failure returned partial decisions: %#v", decisions)
	}
}

func TestRequirementsIsOrderIndependent(t *testing.T) {
	t.Parallel()

	input := []domain.Requirement{
		requirement("root.ext4", domain.Required, domain.SourceFilesystem, "/"),
		requirement("containers", domain.Recommended, domain.SourceProfile, "workstation"),
		requirement("containers", domain.Required, domain.SourcePackage, "app-containers/docker"),
		requirement("intel-wifi", domain.Required, domain.SourceDevice, "0000:00:14.3"),
	}
	want, err := resolve.Requirements(input)
	if err != nil {
		t.Fatalf("resolve baseline: %v", err)
	}

	for seed := range uint64(100) {
		shuffled := append([]domain.Requirement(nil), input...)
		rand.New(rand.NewPCG(seed, seed+1)).Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		got, err := resolve.Requirements(shuffled)
		if err != nil {
			t.Fatalf("seed %d: resolve: %v", seed, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("seed %d produced non-deterministic result:\ngot  %#v\nwant %#v", seed, got, want)
		}
	}
}

func FuzzRequirementsNeverReturnsPartialResultOnError(f *testing.F) {
	f.Add("containers", "required")
	f.Add("../../etc/passwd", "prohibited")

	f.Fuzz(func(t *testing.T, capability, disposition string) {
		input := []domain.Requirement{{
			Capability:  capability,
			Disposition: domain.Disposition(disposition),
			Evidence: domain.Evidence{
				Kind:       domain.SourceProfile,
				Source:     "fuzz",
				Detail:     "generated input",
				Confidence: domain.Low,
			},
		}}
		decisions, err := resolve.Requirements(input)
		if err != nil && decisions != nil {
			t.Fatalf("error %v returned partial decisions %#v", err, decisions)
		}
	})
}

func requirement(
	capability string,
	disposition domain.Disposition,
	kind domain.SourceKind,
	source string,
) domain.Requirement {
	return domain.Requirement{
		Capability:  capability,
		Disposition: disposition,
		Evidence: domain.Evidence{
			Kind:       kind,
			Source:     source,
			Detail:     "test evidence",
			Confidence: domain.Certain,
		},
	}
}
