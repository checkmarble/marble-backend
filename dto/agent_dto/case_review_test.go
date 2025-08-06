package agent_dto

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockProof struct {
	Id          string `json:"id"`
	Type        string `json:"type"`
	IsDataModel bool   `json:"is_data_model"`
	Reason      string `json:"reason"`
}

type MockCaseReviewV1 struct {
	Ok          bool        `json:"ok"`
	Output      string      `json:"output"`
	SanityCheck string      `json:"sanity_check"`
	Thought     string      `json:"thought"`
	Version     string      `json:"version"`
	Proofs      []MockProof `json:"proofs"`
}

func (c MockCaseReviewV1) aiCaseReviewDto() {}

// Mock V2 implementation with completely different structure
type MockCaseReviewV2 struct {
	Status      string      `json:"status"`
	Analysis    string      `json:"analysis"`
	Metadata    string      `json:"metadata"`
	Version     string      `json:"version"`
	Evidence    []MockProof `json:"evidence"`
	RiskScore   int         `json:"risk_score"`
	ProcessedBy string      `json:"processed_by"`
	ProcessedAt string      `json:"processed_at"`
}

func (c MockCaseReviewV2) aiCaseReviewDto() {}

func TestAiCaseReviewOutputDto_MarshalJSON_V1(t *testing.T) {
	// Arrange
	testId := uuid.New()
	reaction := "ok"

	mockCaseReviewV1 := MockCaseReviewV1{
		Ok:          true,
		Output:      "Test output from mock V1",
		SanityCheck: "Test sanity check from mock",
		Thought:     "Test thought from mock",
		Version:     "mock-v1",
		Proofs: []MockProof{
			{
				Id:          "mock-proof-1",
				Type:        "mock-case",
				IsDataModel: false,
				Reason:      "Test reason from mock",
			},
		},
	}

	dto := AiCaseReviewOutputDto{
		Id:              testId,
		Reaction:        &reaction,
		AiCaseReviewDto: mockCaseReviewV1,
	}

	jsonBytes, err := json.Marshal(dto)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)

	assert.Equal(t, testId.String(), result["id"])
	assert.Equal(t, "ok", result["reaction"])
	assert.Equal(t, true, result["ok"])
	assert.Equal(t, "Test output from mock V1", result["output"])
	assert.Equal(t, "Test sanity check from mock", result["sanity_check"])
	assert.Equal(t, "Test thought from mock", result["thought"])
	assert.Equal(t, "mock-v1", result["version"])

	// Check proofs array
	proofs, ok := result["proofs"].([]interface{})
	require.True(t, ok, "proofs should be an array")
	require.Len(t, proofs, 1)

	proof := proofs[0].(map[string]interface{})
	assert.Equal(t, "mock-proof-1", proof["id"])
	assert.Equal(t, "mock-case", proof["type"])
	assert.Equal(t, false, proof["is_data_model"])
	assert.Equal(t, "Test reason from mock", proof["reason"])

	// Ensure no nested AiCaseReviewDto field exists
	assert.NotContains(t, result, "AiCaseReviewDto")
}

func TestAiCaseReviewOutputDto_MarshalJSON_V2(t *testing.T) {
	// Arrange
	testId := uuid.New()
	reaction := "ok"

	mockCaseReviewV2 := MockCaseReviewV2{
		Status:   "approved",
		Analysis: "Comprehensive analysis from mock V2",
		Metadata: "Mock metadata for V2",
		Version:  "mock-v2",
		Evidence: []MockProof{
			{
				Id:          "mock-evidence-1",
				Type:        "mock-evidence-type",
				IsDataModel: true,
				Reason:      "Evidence reason from mock V2",
			},
		},
		RiskScore:   42,
		ProcessedBy: "mock-ai-system-v2",
		ProcessedAt: "2024-01-15T10:30:00Z",
	}

	dto := AiCaseReviewOutputDto{
		Id:              testId,
		Reaction:        &reaction,
		AiCaseReviewDto: mockCaseReviewV2,
	}

	jsonBytes, err := json.Marshal(dto)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)

	assert.Equal(t, testId.String(), result["id"])
	assert.Equal(t, "ok", result["reaction"])

	// Check V2 specific fields (completely different structure)
	assert.Equal(t, "approved", result["status"])
	assert.Equal(t, "Comprehensive analysis from mock V2", result["analysis"])
	assert.Equal(t, "Mock metadata for V2", result["metadata"])
	assert.Equal(t, "mock-v2", result["version"])
	assert.Equal(t, float64(42), result["risk_score"]) // JSON numbers are float64
	assert.Equal(t, "mock-ai-system-v2", result["processed_by"])
	assert.Equal(t, "2024-01-15T10:30:00Z", result["processed_at"])

	// Check evidence array (different from proofs)
	evidence, ok := result["evidence"].([]interface{})
	require.True(t, ok, "evidence should be an array")
	require.Len(t, evidence, 1)

	evidenceItem := evidence[0].(map[string]interface{})
	assert.Equal(t, "mock-evidence-1", evidenceItem["id"])
	assert.Equal(t, "mock-evidence-type", evidenceItem["type"])
	assert.Equal(t, true, evidenceItem["is_data_model"])
	assert.Equal(t, "Evidence reason from mock V2", evidenceItem["reason"])

	// Ensure no nested AiCaseReviewDto field exists
	assert.NotContains(t, result, "AiCaseReviewDto")

	// Ensure V1/V2 fields are not present (proving it's truly different)
	assert.NotContains(t, result, "ok")
	assert.NotContains(t, result, "output")
	assert.NotContains(t, result, "proofs")
	assert.NotContains(t, result, "confidence")
}

func TestAiCaseReviewOutputDto_MarshalJSON_NilReaction(t *testing.T) {
	// Arrange
	testId := uuid.New()

	mockCaseReviewV1 := MockCaseReviewV1{
		Ok:          true,
		Output:      "Test output for nil reaction",
		SanityCheck: "",
		Thought:     "",
		Version:     "mock-v1-nil",
		Proofs:      []MockProof{},
	}

	dto := AiCaseReviewOutputDto{
		Id:              testId,
		Reaction:        nil, // Test nil reaction
		AiCaseReviewDto: mockCaseReviewV1,
	}

	// Act
	jsonBytes, err := json.Marshal(dto)
	require.NoError(t, err)

	// Assert
	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)

	// Check that reaction is null in JSON
	assert.Equal(t, testId.String(), result["id"])
	assert.Nil(t, result["reaction"])
	assert.Equal(t, true, result["ok"])
	assert.Equal(t, "Test output for nil reaction", result["output"])
	assert.Equal(t, "mock-v1-nil", result["version"])
}

// Test that demonstrates adding new fields to AiCaseReviewOutputDto works automatically
func TestAiCaseReviewOutputDto_MarshalJSON_AdditionalFields(t *testing.T) {
	type ExtendedDto struct {
		Id       uuid.UUID `json:"id"`
		Reaction *string   `json:"reaction"`
		Comment  *string   `json:"comment"` // New field
		Score    int       `json:"score"`   // Another new field

		AiCaseReviewDto
	}

	testId := uuid.New()
	reaction := "ok"
	comment := "Great review"
	score := 95

	caseReviewV1 := CaseReviewV1{
		Ok:      true,
		Output:  "Test output",
		Version: "v1",
		Proofs:  []CaseReviewProof{},
	}

	extendedDto := ExtendedDto{
		Id:              testId,
		Reaction:        &reaction,
		Comment:         &comment,
		Score:           score,
		AiCaseReviewDto: caseReviewV1,
	}

	_, err := json.Marshal(extendedDto)
	require.NoError(t, err)

	assert.Equal(t, testId.String(), extendedDto.Id.String())
	assert.Equal(t, reaction, *extendedDto.Reaction)
	assert.Equal(t, comment, *extendedDto.Comment)
	assert.Equal(t, score, extendedDto.Score)
}
