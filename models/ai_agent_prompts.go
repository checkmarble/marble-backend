package models

import (
	"bytes"
	"io/fs"
	"path"
	"time"
)

// Path constants for every prompt file the ai_agent package reads from a prompts filesystem,
// root-relative (see usecases/ai_agent's readPrompt/preparePrompt). Single source of truth,
// shared by infra (fills/validates a downloaded prompts zip against them) and usecases/ai_agent
// (reads them at runtime, validates a resolved prompts fs.FS against them at startup).
const (
	AiAgentModelConfigFileName                                 = "ai_agent_models.json"
	AiAgentSystemPromptPath                                    = "system.md"
	AiAgentCaseReviewPromptPath                                = "case_review/case_review.md"
	AiAgentCaseReviewDataModelObjectFieldReadOptionsPromptPath = "case_review/data_model_object_field_read_options.md"
	AiAgentCaseReviewDataModelSummaryPromptPath                = "case_review/data_model_summary.md"
	AiAgentCaseReviewCustomReportInstructionPath               = "case_review/instruction_custom_report.md"
	AiAgentCaseReviewLanguageInstructionPath                   = "case_review/instruction_language.md"
	AiAgentCaseReviewStructureInstructionPath                  = "case_review/instruction_structure.md"
	AiAgentCaseReviewRuleDefinitionsPromptPath                 = "case_review/rule_definitions.md"
	AiAgentCaseReviewRuleThresholdValuesPromptPath             = "case_review/rule_threshold_values.md"
	AiAgentCaseReviewSanityCheckPromptPath                     = "case_review/sanity_check.md"
	AiAgentKYCInstructionPath                                  = "kyc_enrichment/instruction.md"
	AiAgentKYCPromptEnrichPath                                 = "kyc_enrichment/prompt_enrich.md"
	AiAgentRuleDescriptionPromptPath                           = "rule/rule_description.md"
	AiAgentRuleGenerationStep1PromptPath                       = "rule/rule_generation_step1.md"
	AiAgentRuleGenerationStep2PromptPath                       = "rule/rule_generation_step2.md"
	AiAgentScreeningHitEvaluatePromptPath                      = "screening_hit_suggestion/evaluate_match.md"
	AiAgentScreeningHitSystemPromptPath                        = "screening_hit_suggestion/system.md"
)

// AiAgentPromptPaths is the deterministic, exhaustive manifest of every markdown prompt file
// above - deliberately an explicit list rather than something derived by walking a directory,
// so a missing entry can be caught (at download time, or at startup validation) instead of
// surfacing later as a per-job failure. Does not include AiAgentModelConfigFileName (a separate,
// JSON, not markdown, file) - see AiAgentExpectedFiles for the combined manifest.
var AiAgentExpectedFiles = []string{
	AiAgentModelConfigFileName,
	AiAgentSystemPromptPath,
	AiAgentCaseReviewPromptPath,
	AiAgentCaseReviewDataModelObjectFieldReadOptionsPromptPath,
	AiAgentCaseReviewDataModelSummaryPromptPath,
	AiAgentCaseReviewCustomReportInstructionPath,
	AiAgentCaseReviewLanguageInstructionPath,
	AiAgentCaseReviewStructureInstructionPath,
	AiAgentCaseReviewRuleDefinitionsPromptPath,
	AiAgentCaseReviewRuleThresholdValuesPromptPath,
	AiAgentCaseReviewSanityCheckPromptPath,
	AiAgentKYCInstructionPath,
	AiAgentKYCPromptEnrichPath,
	AiAgentRuleDescriptionPromptPath,
	AiAgentRuleGenerationStep1PromptPath,
	AiAgentRuleGenerationStep2PromptPath,
	AiAgentScreeningHitEvaluatePromptPath,
	AiAgentScreeningHitSystemPromptPath,
}

// AiAgentPromptsMapFS is a minimal, read-only, in-memory fs.FS backed by a name -> content map.
// It exists so a downloaded ai prompts bundle (see infra.InitAiPromptsFS) can be held entirely
// in memory - filled deterministically against AiAgentExpectedFiles, never a generic dump of
// arbitrary content - without touching disk or depending on a testing-only package.
type AiAgentPromptsMapFS map[string][]byte

func (m AiAgentPromptsMapFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	content, ok := m[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &aiAgentPromptsMapFile{Reader: bytes.NewReader(content), name: name, size: int64(len(content))}, nil
}

// ReadFile implements fs.ReadFileFS, the fast path fs.ReadFile uses when available - a plain
// map lookup instead of Open+ReadAll+Close.
func (m AiAgentPromptsMapFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}
	content, ok := m[name]
	if !ok {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrNotExist}
	}
	out := make([]byte, len(content))
	copy(out, content)
	return out, nil
}

type aiAgentPromptsMapFile struct {
	*bytes.Reader
	name string
	size int64
}

func (f *aiAgentPromptsMapFile) Stat() (fs.FileInfo, error) {
	return aiAgentPromptsMapFileInfo{name: f.name, size: f.size}, nil
}

func (f *aiAgentPromptsMapFile) Close() error { return nil }

type aiAgentPromptsMapFileInfo struct {
	name string
	size int64
}

func (fi aiAgentPromptsMapFileInfo) Name() string       { return path.Base(fi.name) }
func (fi aiAgentPromptsMapFileInfo) Size() int64        { return fi.size }
func (fi aiAgentPromptsMapFileInfo) Mode() fs.FileMode  { return 0o444 }
func (fi aiAgentPromptsMapFileInfo) ModTime() time.Time { return time.Time{} }
func (fi aiAgentPromptsMapFileInfo) IsDir() bool        { return false }
func (fi aiAgentPromptsMapFileInfo) Sys() any           { return nil }
