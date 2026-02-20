# AI Rule Generation Endpoint Design

**Date**: 2026-02-20
**Feature**: AI-assisted rule generation from natural language instructions
**Status**: Design Approved

## Overview

Add a UI-integrated endpoint that allows users to generate rule AST expressions using OpenAI based on natural language instructions. The feature provides a "Generate" button in an AI Assistant panel that's always visible while editing rules.

## Goals

1. Enable users to create rules through natural language instead of manual AST construction
2. Validate generated rules structurally before returning to frontend
3. Provide a simple, one-shot generation flow (iterate from there)
4. Use same permission model as rule editing

## API Design

### Endpoint

```
POST /scenario-iteration-rules/:rule_id/generate
Content-Type: application/json
```

### Request

```json
{
  "instruction": "Flag transactions over $1000 from high-risk countries"
}
```

**Parameters:**
- `rule_id` (path): ID of the rule to generate for (used for context, not persisted)
- `instruction` (body): Natural language description of what the rule should do

### Response (Success: 200 or 400)

Always returns AST + validation result, even if validation fails:

```json
{
  "rule_ast": {
    "name": "...",
    "constant": null,
    "children": [...],
    "named_children": {}
  },
  "validation": {
    "is_valid": true,
    "errors": [],
    "warnings": []
  }
}
```

**Status codes:**
- **200**: Generated successfully and is valid
- **400**: Generated but has validation errors (still return AST for frontend to show why it failed)
- **400/500**: Generation failed (LLM error, malformed input, etc.) - return error message only

### Response (Error: 400/500)

```json
{
  "error": "LLM generation failed: context length exceeded"
}
```

## Architecture

### Backend Components

**New method in `AiAgentUsecase`:**

```go
func (uc *AiAgentUsecase) GenerateRule(
  ctx context.Context,
  orgId uuid.UUID,
  ruleId string,
  instruction string,
) (models.GenerateRuleResponse, error)
```

**Flow:**
1. Fetch rule by ID (permission check happens in rule usecase)
2. Fetch scenario iteration and data model
3. Extract available identifiers:
   - Database accessors (linked table fields via recursive traversal)
   - Payload identifiers (direct trigger object fields)
4. Build prompt with:
   - Data model structure
   - Custom lists
   - Available accessors
   - User instruction
5. Call OpenAI with structured schema (recursive NodeDto)
6. Validate generated AST against scenario
7. Return AST + validation result

**HTTP Handler in `api/handle_scenario_iterations.go`:**

- Parse JSON request body
- Extract rule_id from path
- Extract org_id from request
- Call usecase
- Return response

### Data Models

**DTO: GenerateRuleResponse**
```go
type GenerateRuleResponse struct {
  RuleAST    *ast.Node          `json:"rule_ast"`
  Validation ASTValidationResult `json:"validation"`
}
```

**Reuse existing**: `ASTValidationResult` from scenario validation

## Key Design Decisions

### No Persistence
Unlike the current experimental branch, **the endpoint does NOT update the rule in the database**. It returns the generated AST and lets the frontend decide when/if to persist it. This:
- Keeps the endpoint simpler
- Gives the frontend full control (can show preview, ask confirmation, etc.)
- Separates generation from persistence concerns

### Validation Included
Return validation result with the AST, even on failure. This allows the frontend to:
- Show users why a generated rule didn't work
- Display the invalid rule with error highlights
- Let users iterate without losing the generation

### Reuse Existing Identifiers
Use the recursive identifier extraction from the experimental branch (`getLinkedDatabaseIdentifiers`, `getPayloadIdentifiers`). These already handle:
- Circular reference prevention
- Deep nesting through links
- Proper path construction

## Frontend Integration (High-Level)

**UI Location**: Separate "AI Assistant" panel, always visible while editing rules

**User Flow:**
1. User enters instruction in text field
2. Clicks "Generate"
3. Frontend shows loading spinner
4. Backend generates AST
5. Response returns with AST + validation
6. Frontend:
   - If valid: Display generated rule in editor
   - If invalid: Highlight validation errors, show both generated AST and error details
7. User can save, discard, or regenerate with different instruction

## Error Handling

**Validation Errors**: Return 400 with AST + validation details
- Frontend displays why generation failed
- User can iterate with adjusted instruction

**LLM Errors**: Return 400/500 with error message
- Timeout, malformed response, rate limit, etc.
- Frontend shows user-friendly error message

**Permission Errors**: Return 403 (caught by existing permission middleware)
- User cannot edit the scenario iteration

**Not Found**: Return 404
- Rule or scenario doesn't exist

## Testing Strategy

**Unit Tests:**
- Mock LLM client, test prompt construction
- Test identifier extraction with various data models
- Test AST validation integration

**Integration Tests:**
- End-to-end with real database
- Test permission checks
- Test validation results with valid/invalid generated rules

## Future Iterations

This design enables several natural next steps:
1. **Iterative refinement**: Keep conversation history, refine instructions
2. **Preview before accept**: Show generated rule in sidebar before replacing editor
3. **Variant generation**: Generate multiple rule options, user picks one
4. **Rule explanation**: Add endpoint to explain existing rules in natural language
