package ai_agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Mock implementation of AgentPrinter for testing
type mockAgentPrinter struct {
	value string
}

func (m mockAgentPrinter) PrintForAgent() (string, error) {
	return m.value, nil
}

func TestPreparePrompt_StringTypes(t *testing.T) {
	// Create a temporary prompt file
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "test_prompt.txt")
	promptContent := "Name: {{ .name }}\nDescription: {{ .description }}"

	err := os.WriteFile(promptPath, []byte(promptContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Test with string values
	data := map[string]any{
		"name":        "Test Name",
		"description": "Test Description",
	}

	result, err := preparePrompt(promptPath, data)
	if err != nil {
		t.Fatalf("preparePrompt failed: %v", err)
	}

	if !strings.Contains(result, "Name: Test Name") {
		t.Errorf("Expected result to contain 'Name: Test Name', got: %s", result)
	}
	if !strings.Contains(result, "Description: Test Description") {
		t.Errorf("Expected result to contain 'Description: Test Description', got: %s", result)
	}
}

func TestPreparePrompt_StringPointers(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "test_prompt.txt")
	promptContent := "Value: {{ .value }}\nNullValue: {{ .nullValue }}"

	err := os.WriteFile(promptPath, []byte(promptContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Test with string pointer values
	testValue := "pointer value"
	data := map[string]any{
		"value":     &testValue,
		"nullValue": (*string)(nil),
	}

	result, err := preparePrompt(promptPath, data)
	if err != nil {
		t.Fatalf("preparePrompt failed: %v", err)
	}

	if !strings.Contains(result, "Value: pointer value") {
		t.Errorf("Expected result to contain 'Value: pointer value', got: %s", result)
	}
	if !strings.Contains(result, "NullValue: null") {
		t.Errorf("Expected result to contain 'NullValue: null', got: %s", result)
	}
}

func TestPreparePrompt_AgentPrinter(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "test_prompt.txt")
	promptContent := "Printer: {{ .printer }}"

	err := os.WriteFile(promptPath, []byte(promptContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Test with AgentPrinter implementation
	data := map[string]any{
		"printer": mockAgentPrinter{value: "custom printed value"},
	}

	result, err := preparePrompt(promptPath, data)
	if err != nil {
		t.Fatalf("preparePrompt failed: %v", err)
	}

	if !strings.Contains(result, "Printer: custom printed value") {
		t.Errorf("Expected result to contain 'Printer: custom printed value', got: %s", result)
	}
}

func TestPreparePrompt_ComplexTypes(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "test_prompt.txt")
	promptContent := "Data: {{ .data }}"

	err := os.WriteFile(promptPath, []byte(promptContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Test with complex types (maps, structs, etc.)
	data := map[string]any{
		"data": map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		},
	}

	result, err := preparePrompt(promptPath, data)
	if err != nil {
		t.Fatalf("preparePrompt failed: %v", err)
	}

	// The complex type should be JSON encoded
	if !strings.Contains(result, "key1") || !strings.Contains(result, "value1") {
		t.Errorf("Expected result to contain JSON-encoded data, got: %s", result)
	}
}

func TestPreparePrompt_HTMLCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "test_prompt.txt")
	promptContent := "Content: {{ .content }}"

	err := os.WriteFile(promptPath, []byte(promptContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Test that HTML characters are not escaped
	data := map[string]any{
		"content": `<script>alert("test")</script> & "quotes"`,
	}

	result, err := preparePrompt(promptPath, data)
	if err != nil {
		t.Fatalf("preparePrompt failed: %v", err)
	}

	// Should contain unescaped HTML characters
	if !strings.Contains(result, `<script>alert("test")</script>`) {
		t.Errorf("Expected HTML characters to be unescaped, got: %s", result)
	}
}

func TestPreparePrompt_MixedTypes(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "test_prompt.txt")
	promptContent := `String: {{ .str }}
Pointer: {{ .ptr }}
Printer: {{ .printer }}
Complex: {{ .complex }}`

	err := os.WriteFile(promptPath, []byte(promptContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test prompt file: %v", err)
	}

	// Test with mixed types
	ptrValue := "pointer"
	data := map[string]any{
		"str":     "simple string",
		"ptr":     &ptrValue,
		"printer": mockAgentPrinter{value: "printed"},
		"complex": []string{"a", "b", "c"},
	}

	result, err := preparePrompt(promptPath, data)
	if err != nil {
		t.Fatalf("preparePrompt failed: %v", err)
	}

	if !strings.Contains(result, "String: simple string") {
		t.Errorf("Expected string value in result, got: %s", result)
	}
	if !strings.Contains(result, "Pointer: pointer") {
		t.Errorf("Expected pointer value in result, got: %s", result)
	}
	if !strings.Contains(result, "Printer: printed") {
		t.Errorf("Expected printer value in result, got: %s", result)
	}
	if !strings.Contains(result, `Complex: ["a","b","c"]`) {
		t.Errorf("Expected complex value in result, got: %s", result)
	}
}
