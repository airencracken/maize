package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type Disposition string

const (
	Required    Disposition = "required"
	Recommended Disposition = "recommended"
	Optional    Disposition = "optional"
	Prohibited  Disposition = "prohibited"
)

type Confidence string

const (
	Certain Confidence = "certain"
	High    Confidence = "high"
	Medium  Confidence = "medium"
	Low     Confidence = "low"
)

type SourceKind string

const (
	SourceConfig      SourceKind = "config"
	SourceDevice      SourceKind = "device"
	SourceFilesystem  SourceKind = "filesystem"
	SourcePackage     SourceKind = "package"
	SourceProfile     SourceKind = "profile"
	SourceUseFlag     SourceKind = "use-flag"
	SourceOperator    SourceKind = "operator"
	SourceDependency  SourceKind = "dependency"
	SourceKernel      SourceKind = "kernel"
	SourceObservation SourceKind = "observation"
)

type Evidence struct {
	Kind       SourceKind
	Source     string
	Detail     string
	Confidence Confidence
}

// Provenance identifies where a fact came from and, when applicable, when it
// was observed. It contains no collection behavior or process-global state.
type Provenance struct {
	Kind       SourceKind
	Source     string
	Detail     string
	ObservedAt time.Time
}

func (p Provenance) Validate() error {
	if !validSourceKind(p.Kind) {
		return fmt.Errorf("invalid provenance kind %q", p.Kind)
	}
	if strings.TrimSpace(p.Source) == "" {
		return errors.New("provenance source is required")
	}
	if strings.TrimSpace(p.Detail) == "" {
		return errors.New("provenance detail is required")
	}
	return nil
}

// Explanation is the operator-facing reason for a conclusion. Evidence is
// retained separately so renderers can show a concise summary or a full audit.
type Explanation struct {
	Summary    string
	Detail     string
	Confidence Confidence
	Provenance []Provenance
}

func (e Explanation) Validate() error {
	if strings.TrimSpace(e.Summary) == "" {
		return errors.New("explanation summary is required")
	}
	if !validConfidence(e.Confidence) {
		return fmt.Errorf("invalid confidence %q", e.Confidence)
	}
	for index, provenance := range e.Provenance {
		if err := provenance.Validate(); err != nil {
			return fmt.Errorf("provenance %d: %w", index, err)
		}
	}
	return nil
}

func (e Evidence) Validate() error {
	if !validSourceKind(e.Kind) {
		return fmt.Errorf("invalid evidence kind %q", e.Kind)
	}
	if strings.TrimSpace(e.Source) == "" {
		return errors.New("evidence source is required")
	}
	if strings.TrimSpace(e.Detail) == "" {
		return errors.New("evidence detail is required")
	}
	if !validConfidence(e.Confidence) {
		return fmt.Errorf("invalid confidence %q", e.Confidence)
	}
	return nil
}

type Requirement struct {
	Capability  string
	Disposition Disposition
	Evidence    Evidence
}

func (r Requirement) Validate() error {
	if strings.TrimSpace(r.Capability) == "" {
		return errors.New("capability is required")
	}
	if !validCapability(r.Capability) {
		return fmt.Errorf("invalid capability %q", r.Capability)
	}
	if !validDisposition(r.Disposition) {
		return fmt.Errorf("invalid disposition %q", r.Disposition)
	}
	if err := r.Evidence.Validate(); err != nil {
		return fmt.Errorf("evidence: %w", err)
	}
	return nil
}

type Decision struct {
	Capability  string
	Disposition Disposition
	Evidence    []Evidence
}

type Conflict struct {
	Capability   string
	Dispositions []Disposition
	Evidence     []Evidence
}

func validCapability(value string) bool {
	for i, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			continue
		}
		if i > 0 && (r == '-' || r == '.') {
			continue
		}
		return false
	}
	return value != ""
}

func validDisposition(value Disposition) bool {
	return slices.Contains([]Disposition{Required, Recommended, Optional, Prohibited}, value)
}

func validConfidence(value Confidence) bool {
	return slices.Contains([]Confidence{Certain, High, Medium, Low}, value)
}

func validSourceKind(value SourceKind) bool {
	return slices.Contains([]SourceKind{
		SourceConfig,
		SourceDevice,
		SourceFilesystem,
		SourcePackage,
		SourceProfile,
		SourceUseFlag,
		SourceOperator,
		SourceDependency,
		SourceKernel,
		SourceObservation,
	}, value)
}
