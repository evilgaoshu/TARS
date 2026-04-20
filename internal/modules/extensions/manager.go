package extensions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"tars/internal/modules/skills"
)

const (
	BundleAPIVersion = "tars.extension/v1alpha1"
	BundleKind       = "skill_bundle"

	StatusGenerated = "generated"
	StatusValidated = "validated"
	StatusInvalid   = "invalid"
	StatusImported  = "imported"

	ReviewPending          = "pending"
	ReviewChangesRequested = "changes_requested"
	ReviewApproved         = "approved"
	ReviewRejected         = "rejected"
	ReviewImported         = "imported"
)

var (
	ErrCandidateNotFound    = errors.New("extension candidate not found")
	ErrReviewStateRequired  = errors.New("review_state is required")
	ErrInvalidReviewState   = errors.New("review_state is invalid")
	ErrReviewReasonRequired = errors.New("operator_reason is required")
)

type Bundle struct {
	APIVersion    string                     `json:"api_version" yaml:"api_version"`
	Kind          string                     `json:"kind" yaml:"kind"`
	Metadata      BundleMetadata             `json:"metadata" yaml:"metadata"`
	Skill         skills.Manifest            `json:"skill" yaml:"skill"`
	Docs          []DocsAsset                `json:"docs,omitempty" yaml:"docs,omitempty"`
	Tests         []TestSpec                 `json:"tests,omitempty" yaml:"tests,omitempty"`
	Compatibility skills.CompatibilityReport `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
}

type BundleMetadata struct {
	ID          string    `json:"id,omitempty" yaml:"id,omitempty"`
	DisplayName string    `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Summary     string    `json:"summary,omitempty" yaml:"summary,omitempty"`
	Source      string    `json:"source,omitempty" yaml:"source,omitempty"`
	GeneratedBy string    `json:"generated_by,omitempty" yaml:"generated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty" yaml:"created_at,omitempty"`
}

type DocsAsset struct {
	ID      string `json:"id,omitempty" yaml:"id,omitempty"`
	Slug    string `json:"slug,omitempty" yaml:"slug,omitempty"`
	Title   string `json:"title,omitempty" yaml:"title,omitempty"`
	Format  string `json:"format,omitempty" yaml:"format,omitempty"`
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Content string `json:"content,omitempty" yaml:"content,omitempty"`
}

type TestSpec struct {
	ID      string `json:"id,omitempty" yaml:"id,omitempty"`
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Kind    string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
}

type ValidationReport struct {
	Valid     bool      `json:"valid" yaml:"valid"`
	Errors    []string  `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings  []string  `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	CheckedAt time.Time `json:"checked_at,omitempty" yaml:"checked_at,omitempty"`
}

type PreviewSummary struct {
	ChangeType string   `json:"change_type,omitempty" yaml:"change_type,omitempty"`
	Summary    []string `json:"summary,omitempty" yaml:"summary,omitempty"`
}

type ReviewEvent struct {
	State      string    `json:"state,omitempty" yaml:"state,omitempty"`
	Reason     string    `json:"reason,omitempty" yaml:"reason,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	ImportedBy string    `json:"imported_by,omitempty" yaml:"imported_by,omitempty"`
}

type Candidate struct {
	ID               string           `json:"id" yaml:"id"`
	Bundle           Bundle           `json:"bundle" yaml:"bundle"`
	Status           string           `json:"status,omitempty" yaml:"status,omitempty"`
	ReviewState      string           `json:"review_state,omitempty" yaml:"review_state,omitempty"`
	ReviewHistory    []ReviewEvent    `json:"review_history,omitempty" yaml:"review_history,omitempty"`
	Validation       ValidationReport `json:"validation" yaml:"validation"`
	Preview          PreviewSummary   `json:"preview" yaml:"preview"`
	ImportedSkillID  string           `json:"imported_skill_id,omitempty" yaml:"imported_skill_id,omitempty"`
	ImportedAt       time.Time        `json:"imported_at,omitempty" yaml:"imported_at,omitempty"`
	LastOperatorNote string           `json:"last_operator_note,omitempty" yaml:"last_operator_note,omitempty"`
	CreatedAt        time.Time        `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt        time.Time        `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

type GenerateOptions struct {
	Bundle         Bundle
	OperatorReason string
}

type ReviewOptions struct {
	State          string
	OperatorReason string
}

type ImportResult struct {
	Candidate Candidate
	Manifest  skills.Manifest
	State     skills.LifecycleState
}

type stateFile struct {
	Extensions struct {
		Entries []Candidate `yaml:"entries,omitempty"`
	} `yaml:"extensions"`
}

type Manager struct {
	mu         sync.RWMutex
	path       string
	skills     *skills.Manager
	candidates map[string]Candidate
	loadedAt   time.Time
}

func NewManager(path string, skillManager *skills.Manager) (*Manager, error) {
	manager := &Manager{path: strings.TrimSpace(path), skills: skillManager, candidates: map[string]Candidate{}}
	if err := manager.Reload(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) Reload() error {
	if m == nil {
		return nil
	}
	entries, err := loadCandidates(m.path)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	m.mu.Lock()
	m.candidates = entries
	m.loadedAt = now
	m.mu.Unlock()
	return nil
}

func (m *Manager) List() []Candidate {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]Candidate, 0, len(m.candidates))
	for _, item := range m.candidates {
		items = append(items, cloneCandidate(item))
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}

func (m *Manager) Get(id string) (Candidate, bool) {
	if m == nil {
		return Candidate{}, false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Candidate{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.candidates[id]
	if !ok {
		return Candidate{}, false
	}
	return cloneCandidate(item), true
}

func (m *Manager) Generate(opts GenerateOptions) (Candidate, error) {
	if m == nil {
		return Candidate{}, errors.New("extension manager is not configured")
	}
	bundle, validation, preview, err := m.normalizeAndValidate(opts.Bundle, opts.OperatorReason)
	if err != nil {
		return Candidate{}, err
	}
	now := time.Now().UTC()
	status := StatusGenerated
	if validation.Valid {
		status = StatusValidated
	} else {
		status = StatusInvalid
	}
	candidate := Candidate{
		ID:          uuid.NewString(),
		Bundle:      bundle,
		Status:      status,
		ReviewState: ReviewPending,
		ReviewHistory: []ReviewEvent{{
			State:     ReviewPending,
			Reason:    firstNonEmpty(strings.TrimSpace(opts.OperatorReason), "candidate generated"),
			CreatedAt: now,
		}},
		Validation:       validation,
		Preview:          preview,
		LastOperatorNote: strings.TrimSpace(opts.OperatorReason),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := m.persistCandidate(candidate); err != nil {
		return Candidate{}, err
	}
	return cloneCandidate(candidate), nil
}

func (m *Manager) ValidateCandidate(id string) (Candidate, error) {
	current, ok := m.Get(id)
	if !ok {
		return Candidate{}, ErrCandidateNotFound
	}
	bundle, validation, preview, err := m.normalizeAndValidate(current.Bundle, current.LastOperatorNote)
	if err != nil {
		return Candidate{}, err
	}
	current.Bundle = bundle
	current.Validation = validation
	current.Preview = preview
	current.UpdatedAt = time.Now().UTC()
	if validation.Valid {
		if current.Status != StatusImported {
			current.Status = StatusValidated
		}
	} else {
		current.Status = StatusInvalid
	}
	if err := m.persistCandidate(current); err != nil {
		return Candidate{}, err
	}
	return cloneCandidate(current), nil
}

func (m *Manager) ValidateBundle(bundle Bundle) (Bundle, ValidationReport, PreviewSummary, error) {
	return m.normalizeAndValidate(bundle, "")
}

func (m *Manager) ReviewCandidate(id string, opts ReviewOptions) (Candidate, error) {
	if m == nil {
		return Candidate{}, errors.New("extension manager is not configured")
	}
	current, ok := m.Get(id)
	if !ok {
		return Candidate{}, ErrCandidateNotFound
	}
	state := normalizeReviewState(opts.State)
	if state == "" {
		return Candidate{}, ErrReviewStateRequired
	}
	if !IsAllowedReviewState(state) {
		return Candidate{}, ErrInvalidReviewState
	}
	reason := strings.TrimSpace(opts.OperatorReason)
	if reason == "" {
		return Candidate{}, ErrReviewReasonRequired
	}
	if current.Status == StatusImported {
		return Candidate{}, errors.New("imported candidate review state is immutable")
	}
	current.ReviewState = state
	current.LastOperatorNote = reason
	current.UpdatedAt = time.Now().UTC()
	current.ReviewHistory = append(current.ReviewHistory, ReviewEvent{State: state, Reason: reason, CreatedAt: current.UpdatedAt})
	if err := m.persistCandidate(current); err != nil {
		return Candidate{}, err
	}
	return cloneCandidate(current), nil
}

func (m *Manager) ImportCandidate(id string, operatorReason string) (ImportResult, error) {
	if m == nil {
		return ImportResult{}, errors.New("extension manager is not configured")
	}
	if m.skills == nil {
		return ImportResult{}, errors.New("skill manager is not configured")
	}
	candidate, err := m.ValidateCandidate(id)
	if err != nil {
		return ImportResult{}, err
	}
	if !candidate.Validation.Valid {
		return ImportResult{}, errors.New("extension candidate is not valid")
	}
	if candidate.ReviewState != ReviewApproved {
		return ImportResult{}, errors.New("extension candidate must be approved before import")
	}
	source := firstNonEmpty(candidate.Bundle.Skill.Metadata.Source, candidate.Bundle.Metadata.Source, "extension_bundle")
	manifest, state, err := m.skills.Upsert(skills.UpsertOptions{
		Manifest: candidate.Bundle.Skill,
		Reason:   firstNonEmpty(strings.TrimSpace(operatorReason), "import extension bundle"),
		Action:   "skill_imported",
		Source:   source,
		Status:   "draft",
	})
	if err != nil {
		return ImportResult{}, err
	}
	now := time.Now().UTC()
	candidate.Status = StatusImported
	candidate.ReviewState = ReviewImported
	candidate.ImportedSkillID = manifest.Metadata.ID
	candidate.ImportedAt = now
	candidate.LastOperatorNote = strings.TrimSpace(operatorReason)
	candidate.UpdatedAt = now
	candidate.ReviewHistory = append(candidate.ReviewHistory, ReviewEvent{
		State:      ReviewImported,
		Reason:     firstNonEmpty(strings.TrimSpace(operatorReason), "extension imported"),
		CreatedAt:  now,
		ImportedBy: manifest.Metadata.ID,
	})
	if err := m.persistCandidate(candidate); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Candidate: cloneCandidate(candidate), Manifest: manifest, State: state}, nil
}

func (m *Manager) normalizeAndValidate(bundle Bundle, operatorReason string) (Bundle, ValidationReport, PreviewSummary, error) {
	normalized, err := normalizeBundle(bundle)
	if err != nil {
		return Bundle{}, ValidationReport{}, PreviewSummary{}, err
	}
	validation := buildValidationReport(normalized, operatorReason)
	preview := m.buildPreview(normalized)
	return normalized, validation, preview, nil
}

func normalizeBundle(bundle Bundle) (Bundle, error) {
	bundle.APIVersion = firstNonEmpty(strings.TrimSpace(bundle.APIVersion), BundleAPIVersion)
	if bundle.APIVersion != BundleAPIVersion {
		return Bundle{}, errors.New("extension api_version must be tars.extension/v1alpha1")
	}
	bundle.Kind = firstNonEmpty(strings.TrimSpace(bundle.Kind), BundleKind)
	if bundle.Kind != BundleKind {
		return Bundle{}, errors.New("extension kind must be skill_bundle")
	}
	payload, err := yaml.Marshal(bundle.Skill)
	if err != nil {
		return Bundle{}, err
	}
	manifest, _, err := skills.ParseManifest(payload)
	if err != nil {
		return Bundle{}, err
	}
	bundle.Skill = *manifest
	bundle.Metadata.ID = strings.TrimSpace(bundle.Metadata.ID)
	if bundle.Metadata.ID == "" {
		bundle.Metadata.ID = bundle.Skill.Metadata.ID
	}
	bundle.Metadata.DisplayName = firstNonEmpty(strings.TrimSpace(bundle.Metadata.DisplayName), bundle.Skill.Metadata.DisplayName, bundle.Skill.Metadata.Name, bundle.Skill.Metadata.ID)
	bundle.Metadata.Summary = strings.TrimSpace(bundle.Metadata.Summary)
	bundle.Metadata.Source = firstNonEmpty(strings.TrimSpace(bundle.Metadata.Source), bundle.Skill.Metadata.Source, "generated")
	bundle.Metadata.GeneratedBy = strings.TrimSpace(bundle.Metadata.GeneratedBy)
	if bundle.Metadata.CreatedAt.IsZero() {
		bundle.Metadata.CreatedAt = time.Now().UTC()
	}
	bundle.Docs = normalizeDocs(bundle.Docs)
	bundle.Tests = normalizeTests(bundle.Tests)
	bundle.Compatibility = skills.CompatibilityReportForManifest(bundle.Skill)
	return bundle, nil
}

func normalizeDocs(input []DocsAsset) []DocsAsset {
	if len(input) == 0 {
		return nil
	}
	items := make([]DocsAsset, 0, len(input))
	for _, item := range input {
		normalized := DocsAsset{
			ID:      strings.TrimSpace(item.ID),
			Slug:    strings.TrimSpace(item.Slug),
			Title:   strings.TrimSpace(item.Title),
			Format:  firstNonEmpty(strings.TrimSpace(item.Format), "markdown"),
			Summary: strings.TrimSpace(item.Summary),
			Content: strings.TrimSpace(item.Content),
		}
		if normalized.ID == "" {
			normalized.ID = firstNonEmpty(normalized.Slug, normalized.Title)
		}
		if normalized.Slug == "" {
			normalized.Slug = normalized.ID
		}
		items = append(items, normalized)
	}
	return items
}

func normalizeTests(input []TestSpec) []TestSpec {
	if len(input) == 0 {
		return nil
	}
	items := make([]TestSpec, 0, len(input))
	for _, item := range input {
		normalized := TestSpec{
			ID:      strings.TrimSpace(item.ID),
			Name:    strings.TrimSpace(item.Name),
			Kind:    firstNonEmpty(strings.TrimSpace(item.Kind), "manual"),
			Command: strings.TrimSpace(item.Command),
		}
		if normalized.ID == "" {
			normalized.ID = firstNonEmpty(normalized.Name, normalized.Command)
		}
		items = append(items, normalized)
	}
	return items
}

func buildValidationReport(bundle Bundle, operatorReason string) ValidationReport {
	errorsList := make([]string, 0, 4)
	warnings := make([]string, 0, 4)
	if strings.TrimSpace(bundle.Metadata.ID) == "" {
		errorsList = append(errorsList, "bundle metadata.id is required")
	}
	if strings.TrimSpace(bundle.Skill.Metadata.ID) == "" {
		errorsList = append(errorsList, "skill metadata.id is required")
	}
	if !bundle.Compatibility.Compatible {
		errorsList = append(errorsList, bundle.Compatibility.Reasons...)
	}
	if strings.TrimSpace(bundle.Skill.Metadata.Content) == "" {
		warnings = append(warnings, "bundle has no instruction content (SKILL.md is empty)")
	}
	if len(bundle.Docs) == 0 {
		warnings = append(warnings, "bundle has no docs assets")
	}
	if len(bundle.Tests) == 0 {
		warnings = append(warnings, "bundle has no test metadata")
	}
	if strings.TrimSpace(operatorReason) == "" {
		warnings = append(warnings, "operator_reason was not recorded during validation")
	}
	return ValidationReport{
		Valid:     len(errorsList) == 0,
		Errors:    cloneStrings(errorsList),
		Warnings:  cloneStrings(warnings),
		CheckedAt: time.Now().UTC(),
	}
}

func (m *Manager) buildPreview(bundle Bundle) PreviewSummary {
	preview := PreviewSummary{
		ChangeType: "create",
		Summary:    []string{},
	}
	if m != nil && m.skills != nil {
		if existing, ok := m.skills.Get(bundle.Skill.Metadata.ID); ok {
			preview.ChangeType = "update"
			preview.Summary = append(preview.Summary, "updates existing skill")
			if existing.Metadata.Description != bundle.Skill.Metadata.Description {
				preview.Summary = append(preview.Summary, "updates skill description")
			}
			if existing.Metadata.Content != bundle.Skill.Metadata.Content {
				preview.Summary = append(preview.Summary, "instruction content updates")
			}
			if !sameStrings(existing.Metadata.Tags, bundle.Skill.Metadata.Tags) {
				preview.Summary = append(preview.Summary, "discovery tags change")
			}
		}
	}
	if preview.ChangeType == "create" {
		preview.Summary = append(preview.Summary, "creates a new governed skill candidate")
	}
	if len(bundle.Docs) > 0 {
		preview.Summary = append(preview.Summary, "includes "+itoa(len(bundle.Docs))+" docs assets")
	}
	if len(bundle.Tests) > 0 {
		preview.Summary = append(preview.Summary, "declares "+itoa(len(bundle.Tests))+" test checks")
	}
	return preview
}

func loadCandidates(path string) (map[string]Candidate, error) {
	if strings.TrimSpace(path) == "" {
		return map[string]Candidate{}, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Candidate{}, nil
		}
		return nil, err
	}
	var raw stateFile
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, err
	}
	items := make(map[string]Candidate, len(raw.Extensions.Entries))
	for _, item := range raw.Extensions.Entries {
		items[item.ID] = cloneCandidate(item)
	}
	return items, nil
}

func (m *Manager) persistCandidate(candidate Candidate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.candidates == nil {
		m.candidates = map[string]Candidate{}
	}
	m.candidates[candidate.ID] = cloneCandidate(candidate)
	if strings.TrimSpace(m.path) == "" {
		return nil
	}
	return saveCandidates(m.path, m.candidates)
}

func saveCandidates(path string, candidates map[string]Candidate) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	var raw stateFile
	ids := make([]string, 0, len(candidates))
	for id := range candidates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		raw.Extensions.Entries = append(raw.Extensions.Entries, cloneCandidate(candidates[id]))
	}
	content, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return writeFileAtomically(path, string(content))
}

func writeFileAtomically(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".extensions-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func normalizeReviewState(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func IsAllowedReviewState(value string) bool {
	switch value {
	case ReviewPending, ReviewChangesRequested, ReviewApproved, ReviewRejected:
		return true
	default:
		return false
	}
}

func cloneCandidate(input Candidate) Candidate {
	cloned := input
	cloned.Bundle = cloneBundle(input.Bundle)
	cloned.Validation.Errors = cloneStrings(input.Validation.Errors)
	cloned.Validation.Warnings = cloneStrings(input.Validation.Warnings)
	cloned.Preview.Summary = cloneStrings(input.Preview.Summary)
	if len(input.ReviewHistory) > 0 {
		cloned.ReviewHistory = append([]ReviewEvent(nil), input.ReviewHistory...)
	}
	return cloned
}

func cloneBundle(input Bundle) Bundle {
	cloned := input
	cloned.Skill = cloneSkillManifest(input.Skill)
	if len(input.Docs) > 0 {
		cloned.Docs = append([]DocsAsset(nil), input.Docs...)
	}
	if len(input.Tests) > 0 {
		cloned.Tests = append([]TestSpec(nil), input.Tests...)
	}
	cloned.Compatibility.Reasons = cloneStrings(input.Compatibility.Reasons)
	return cloned
}

func cloneSkillManifest(input skills.Manifest) skills.Manifest {
	payload, err := yaml.Marshal(input)
	if err != nil {
		return input
	}
	parsed, _, err := skills.ParseManifest(payload)
	if err != nil || parsed == nil {
		return input
	}
	return *parsed
}

func cloneStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	return append([]string(nil), input...)
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if strings.TrimSpace(left[i]) != strings.TrimSpace(right[i]) {
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func DefaultStatePath(skillConfigPath string) string {
	skillConfigPath = strings.TrimSpace(skillConfigPath)
	if skillConfigPath == "" {
		return ""
	}
	return fmt.Sprintf("%s.extensions.state.yaml", skillConfigPath)
}
