# Public API: Case Tags

## Context

The public API currently returns tags on cases as `Ref` objects (id + name), but provides no way to:
- Discover available tags (and their IDs)
- Add or remove tags on a case

This design adds three endpoints to the v1beta public API.

## Endpoints

### GET /v1beta/tags

List all tags for the organization.

**Query parameters:**
- `target` (optional, string, enum: `case` | `object`) — filter by tag target type
- `after` (optional, uuid) — cursor for pagination
- `order` (optional, enum: `ASC` | `DESC`)
- `limit` (optional, int, 1-100)

**Response:**
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "Fraud",
      "target": "case"
    }
  ],
  "pagination": {
    "has_more": true,
    "next_page_id": "uuid"
  }
}
```

### POST /v1beta/cases/:caseId/tags

Add one or more tags to a case. Idempotent: adding an already-present tag is a no-op.

**Constraint:** All tag IDs must reference tags with `target=case`. Returns 400 if any tag has a different target.

**Request body:**
```json
{
  "tag_ids": ["uuid1", "uuid2"]
}
```

**Response:** 200 with the updated case (same shape as `GET /cases/:caseId`).

### DELETE /v1beta/cases/:caseId/tags/:tagId

Remove a tag from a case. Idempotent: removing an absent tag is a no-op.

**Response:** 204 No Content.

## Concurrency

Both write operations use `FOR UPDATE` locking on the case row:

1. Begin transaction
2. `SELECT ... FROM cases WHERE id = :caseId FOR UPDATE`
3. Read current case tags
4. Compute desired state (add new / remove specified)
5. Apply changes via existing repository methods
6. Commit

This prevents race conditions when multiple API clients modify tags concurrently.

## Implementation

### DTO

New `Tag` struct in `pubapi/v1/dto/`:
```go
type Tag struct {
    Id     string `json:"id"`
    Name   string `json:"name"`
    Target string `json:"target"`
}
```

### Usecase changes

New methods on `CaseUseCase`:
- `AddCaseTags(ctx, caseId, tagIds)` — reads current tags with lock, validates target=case, merges, delegates to existing repository methods
- `RemoveCaseTag(ctx, caseId, tagId)` — reads current tags with lock, removes, delegates

Tag listing reuses existing `ListAllTags` from `TagUseCase` (add pagination support).

### Repository changes

- Add `FOR UPDATE` variant for case fetching (or add a `forUpdate` parameter to existing method)
- Add pagination support to `ListOrganizationTags` if not already present

### Validation

- All tag IDs in the add request must exist and have `target=case`
- The case must exist and belong to the organization
- Standard API key authentication and org scoping
