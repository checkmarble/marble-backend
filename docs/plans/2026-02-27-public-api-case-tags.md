# Public API Case Tags — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add v1beta endpoints to list tags, add tags to a case, and remove a tag from a case.

**Architecture:** Three new handlers in `pubapi/v1/` reusing existing usecases. The tag listing endpoint requires pagination support added to the repository and usecase. The add/remove endpoints create new usecase methods that lock the case row with `FOR UPDATE` before modifying tags.

**Tech Stack:** Go, Gin, PostgreSQL, squirrel query builder

---

### Task 1: Add pagination to tag listing (repository layer)

**Files:**
- Modify: `repositories/tag_repository.go:13-34` (ListOrganizationTags)
- Modify: `usecases/tag_usecase.go:16-18` (TagUseCaseRepository interface)

**Step 1: Update `ListOrganizationTags` to accept pagination and make target optional**

In `repositories/tag_repository.go`, replace the `ListOrganizationTags` method:

```go
func (repo *MarbleDbRepository) ListOrganizationTags(ctx context.Context, exec Executor,
	organizationId uuid.UUID, target models.TagTarget, withCaseCount bool,
	pagination *models.PaginationAndSorting,
) ([]models.Tag, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SelectTagColumn...).
		From(fmt.Sprintf("%s AS t", dbmodels.TABLE_TAGS)).
		Where(squirrel.Eq{"org_id": organizationId}).
		Where(squirrel.Eq{"deleted_at": nil}).
		OrderBy("t.created_at DESC", "t.id DESC")

	if target != models.TagTargetUnknown {
		query = query.Where(squirrel.Eq{"target": target})
	}

	if pagination != nil {
		if pagination.OffsetId != "" {
			offsetTag, err := repo.GetTagById(ctx, exec, pagination.OffsetId)
			if err != nil {
				return nil, errors.Wrap(err, "invalid pagination offset")
			}
			if pagination.Order == models.SortingOrderAsc {
				query = query.Where(squirrel.Or{
					squirrel.Gt{"t.created_at": offsetTag.CreatedAt},
					squirrel.And{
						squirrel.Eq{"t.created_at": offsetTag.CreatedAt},
						squirrel.Gt{"t.id": offsetTag.Id},
					},
				})
			} else {
				query = query.Where(squirrel.Or{
					squirrel.Lt{"t.created_at": offsetTag.CreatedAt},
					squirrel.And{
						squirrel.Eq{"t.created_at": offsetTag.CreatedAt},
						squirrel.Lt{"t.id": offsetTag.Id},
					},
				})
			}
		}
		query = query.Limit(uint64(pagination.Limit))
	}

	if target == models.TagTargetCase && withCaseCount {
		query = query.Column("(SELECT count(distinct ct.case_id) FROM " +
			dbmodels.TABLE_CASE_TAGS + " AS ct WHERE ct.tag_id = t.id AND ct.deleted_at IS NULL) AS cases_count")
		return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptTagWithCasesCount)
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptTag)
}
```

You will need to add `"github.com/cockroachdb/errors"` to the import block if not already present.

**Step 2: Update the `TagUseCaseRepository` interface in `usecases/tag_usecase.go`**

Change the `ListOrganizationTags` signature to add the pagination parameter:

```go
ListOrganizationTags(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID,
    target models.TagTarget, withCaseCount bool, pagination *models.PaginationAndSorting) ([]models.Tag, error)
```

**Step 3: Update all callers of `ListOrganizationTags`**

Search for all call sites of `ListOrganizationTags` and `ListAllTags`. Each existing caller should pass `nil` for the pagination parameter. There are at least:
- `usecases/tag_usecase.go:37` — `ListAllTags` method
- Any other callers in the codebase

In `usecases/tag_usecase.go`, update the `ListAllTags` method:

```go
func (usecase *TagUseCase) ListAllTags(ctx context.Context, organizationId uuid.UUID,
	target models.TagTarget, withCaseCount bool,
) ([]models.Tag, error) {
	tags, err := usecase.repository.ListOrganizationTags(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, target, withCaseCount, nil)
```

**Step 4: Add paginated listing method to `TagUseCase`**

Add this new method to `usecases/tag_usecase.go`:

```go
func (usecase *TagUseCase) ListTagsPaginated(ctx context.Context, organizationId uuid.UUID,
	target models.TagTarget, pagination models.PaginationAndSorting,
) ([]models.Tag, bool, error) {
	// Fetch one extra to determine if there's a next page
	paginationWithExtra := pagination
	paginationWithExtra.Limit++

	tags, err := usecase.repository.ListOrganizationTags(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, target, false, &paginationWithExtra)
	if err != nil {
		return nil, false, err
	}

	for _, t := range tags {
		if err := usecase.enforceSecurity.ReadTag(t); err != nil {
			return nil, false, err
		}
	}

	hasNextPage := len(tags) > pagination.Limit
	if hasNextPage {
		tags = tags[:pagination.Limit]
	}

	return tags, hasNextPage, nil
}
```

**Step 5: Verify the project compiles**

Run: `go build ./...`
Expected: SUCCESS (no compilation errors)

**Step 6: Commit**

```
feat: add pagination support to tag listing repository and usecase
```

---

### Task 2: Add `GetCaseByIdForUpdate` repository method

**Files:**
- Modify: `repositories/case_repository.go` (add new method near `GetCaseById`)

**Step 1: Add `GetCaseByIdForUpdate` method**

Add this method right after `GetCaseById` in `repositories/case_repository.go` (after line ~235):

```go
func (repo *MarbleDbRepository) GetCaseByIdForUpdate(ctx context.Context, exec Executor, caseId string) (models.CaseMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.CaseMetadata{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseColumn...).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{"id": caseId}).
		Suffix("FOR UPDATE")

	return SqlToModel(ctx, exec, query, dbmodels.AdaptCaseMetadata)
}
```

Check how `AdaptCaseMetadata` is defined — it should already exist given `GetCaseMetadataById` uses it. If it uses `dbmodels.AdaptCase` instead, check and use the same adapter.

**Step 2: Verify the project compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 3: Commit**

```
feat: add GetCaseByIdForUpdate repository method for row-level locking
```

---

### Task 3: Add `AddCaseTags` and `RemoveCaseTag` usecase methods

**Files:**
- Modify: `usecases/case_usecase.go` (add new methods near existing `CreateCaseTags`)
- Modify: `usecases/case_usecase.go:28` (add `GetCaseByIdForUpdate` to `CaseUseCaseRepository` interface)

**Step 1: Add `GetCaseByIdForUpdate` to the repository interface**

In `usecases/case_usecase.go`, add to the `CaseUseCaseRepository` interface (near line 32, next to `GetCaseById`):

```go
GetCaseByIdForUpdate(ctx context.Context, exec repositories.Executor, caseId string) (models.CaseMetadata, error)
```

**Step 2: Add `AddCaseTags` method**

Add this method after the existing `CreateCaseTags` method (after line ~1185):

```go
func (usecase *CaseUseCase) AddCaseTags(ctx context.Context, caseId string, tagIds []string) (models.Case, error) {
	webhookEventId := uuid.New().String()

	updatedCase, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Case, error) {
		// Lock the case row to prevent concurrent tag modifications
		caseMeta, err := usecase.repository.GetCaseByIdForUpdate(ctx, tx, caseId)
		if err != nil {
			return models.Case{}, err
		}

		availableInboxIds, err := usecase.getAvailableInboxIds(
			ctx, usecase.executorFactory.NewExecutor(), caseMeta.OrganizationId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.enforceSecurity.ReadOrUpdateCase(caseMeta, availableInboxIds); err != nil {
			return models.Case{}, err
		}

		previousCaseTags, err := usecase.repository.ListCaseTagsByCaseId(ctx, tx, caseId)
		if err != nil {
			return models.Case{}, err
		}
		previousTagIds := pure_utils.Map(previousCaseTags,
			func(caseTag models.CaseTag) string { return caseTag.TagId })

		// Only add tags that are not already present
		added := false
		for _, tagId := range tagIds {
			if !slices.Contains(previousTagIds, tagId) {
				if err := usecase.createCaseTag(ctx, tx, caseId, tagId); err != nil {
					return models.Case{}, err
				}
				added = true
			}
		}

		if !added {
			// No changes needed, return current case
			return usecase.repository.GetCaseById(ctx, tx, caseId)
		}

		newTagIds := append(slices.Clone(previousTagIds), pure_utils.Filter(tagIds, func(id string) bool {
			return !slices.Contains(previousTagIds, id)
		})...)

		previousValue := strings.Join(previousTagIds, ",")
		newValue := strings.Join(newTagIds, ",")
		_, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:         caseMeta.OrganizationId,
			CaseId:        caseId,
			EventType:     models.CaseTagsUpdated,
			PreviousValue: &previousValue,
			NewValue:      &newValue,
		})
		if err != nil {
			return models.Case{}, err
		}

		updatedCase, err := usecase.getCaseWithDetails(ctx, tx, caseId)
		if err != nil {
			return models.Case{}, err
		}

		c, err := usecase.repository.GetCaseById(ctx, tx, caseId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return models.Case{}, err
		}

		err = usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: updatedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseTagsUpdated(updatedCase),
		})
		if err != nil {
			return models.Case{}, err
		}

		return updatedCase, nil
	})
	if err != nil {
		return models.Case{}, err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)

	tracking.TrackEvent(ctx, models.AnalyticsCaseTagsUpdated, map[string]interface{}{
		"case_id": updatedCase.Id,
	})
	return updatedCase, nil
}
```

**Step 3: Add `RemoveCaseTag` method**

Add this method right after `AddCaseTags`:

```go
func (usecase *CaseUseCase) RemoveCaseTag(ctx context.Context, caseId string, tagId string) error {
	webhookEventId := uuid.New().String()

	var updatedCase models.Case
	err := executor_factory.TransactionFactory.Transaction(usecase.transactionFactory, ctx, func(
		tx repositories.Transaction,
	) error {
		// Lock the case row to prevent concurrent tag modifications
		caseMeta, err := usecase.repository.GetCaseByIdForUpdate(ctx, tx, caseId)
		if err != nil {
			return err
		}

		availableInboxIds, err := usecase.getAvailableInboxIds(
			ctx, usecase.executorFactory.NewExecutor(), caseMeta.OrganizationId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.ReadOrUpdateCase(caseMeta, availableInboxIds); err != nil {
			return err
		}

		previousCaseTags, err := usecase.repository.ListCaseTagsByCaseId(ctx, tx, caseId)
		if err != nil {
			return err
		}

		// Find the case_tag to delete
		var caseTagToDelete *models.CaseTag
		for _, ct := range previousCaseTags {
			if ct.TagId == tagId {
				caseTagToDelete = &ct
				break
			}
		}

		if caseTagToDelete == nil {
			// Tag not on case, idempotent no-op
			return nil
		}

		if err := usecase.repository.SoftDeleteCaseTag(ctx, tx, caseTagToDelete.Id); err != nil {
			return err
		}

		previousTagIds := pure_utils.Map(previousCaseTags,
			func(ct models.CaseTag) string { return ct.TagId })
		newTagIds := pure_utils.Filter(previousTagIds, func(id string) bool {
			return id != tagId
		})

		previousValue := strings.Join(previousTagIds, ",")
		newValue := strings.Join(newTagIds, ",")
		_, err = usecase.repository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:         caseMeta.OrganizationId,
			CaseId:        caseId,
			EventType:     models.CaseTagsUpdated,
			PreviousValue: &previousValue,
			NewValue:      &newValue,
		})
		if err != nil {
			return err
		}

		c, err := usecase.repository.GetCaseById(ctx, tx, caseId)
		if err != nil {
			return err
		}
		if err := usecase.PerformCaseActionSideEffects(ctx, tx, c); err != nil {
			return err
		}

		updatedCase, err = usecase.getCaseWithDetails(ctx, tx, caseId)
		if err != nil {
			return err
		}

		return usecase.webhookEventsUsecase.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: updatedCase.OrganizationId,
			EventContent:   models.NewWebhookEventCaseTagsUpdated(updatedCase),
		})
	})
	if err != nil {
		return err
	}

	usecase.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookEventId)

	tracking.TrackEvent(ctx, models.AnalyticsCaseTagsUpdated, map[string]interface{}{
		"case_id": updatedCase.Id,
	})
	return nil
}
```

**Step 4: Verify the project compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 5: Commit**

```
feat: add AddCaseTags and RemoveCaseTag usecase methods with FOR UPDATE locking
```

---

### Task 4: Add public API DTO for tags

**Files:**
- Create: `pubapi/v1/dto/tag.go`

**Step 1: Create the Tag DTO**

Create `pubapi/v1/dto/tag.go`:

```go
package dto

import "github.com/checkmarble/marble-backend/models"

type Tag struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Target string `json:"target"`
}

func AdaptTag(t models.Tag) Tag {
	return Tag{
		Id:     t.Id,
		Name:   t.Name,
		Target: string(t.Target),
	}
}
```

**Step 2: Verify the project compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 3: Commit**

```
feat: add Tag DTO for public API
```

---

### Task 5: Add handler for listing tags

**Files:**
- Create: `pubapi/v1/tags.go`
- Modify: `pubapi/v1/routes.go` (register the route in `BetaRoutes`)

**Step 1: Create the tags handler file**

Create `pubapi/v1/tags.go`:

```go
package v1

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

var tagPaginationDefaults = models.PaginationDefaults{
	Limit:  50,
	SortBy: models.TagsSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

type ListTagsParams struct {
	types.PaginationParams
	Target string `form:"target" binding:"omitempty,oneof=case object"`
}

func HandleListTags(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var p ListTagsParams
		if err := c.ShouldBindQuery(&p); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		target := models.TagTargetFromString(p.Target)
		pagination := p.PaginationParams.ToModel(tagPaginationDefaults)

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		tagUsecase := uc.NewTagUseCase()

		tags, hasNextPage, err := tagUsecase.ListTagsPaginated(ctx, orgId, target, pagination)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		nextPageId := ""
		if len(tags) > 0 {
			nextPageId = tags[len(tags)-1].Id
		}

		types.
			NewResponse(pure_utils.Map(tags, dto.AdaptTag)).
			WithPagination(hasNextPage, nextPageId).
			Serve(c)
	}
}
```

Note: You will need to check if `models.TagsSortingCreatedAt` exists. If not, you'll need to add it. Check `models/pagination.go` or similar for the existing sorting constants. If the sorting constants are just strings like `"created_at"`, define it accordingly. If the pagination defaults don't require a `SortBy` field for this use case (since tags always sort by `created_at`), you can use whatever constant the existing pattern expects.

**Step 2: Register the route in `BetaRoutes`**

In `pubapi/v1/routes.go`, add inside the `BetaRoutes` function, in the `root` group (after the continuous screening routes, around line 79):

```go
root.GET("/tags", HandleListTags(uc))
```

**Step 3: Verify the project compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```
feat: add GET /v1beta/tags endpoint for listing tags
```

---

### Task 6: Add handlers for adding/removing case tags

**Files:**
- Modify: `pubapi/v1/tags.go` (add the two new handlers)
- Modify: `pubapi/v1/routes.go` (register the routes)

**Step 1: Add the add-tags handler to `pubapi/v1/tags.go`**

Append to `pubapi/v1/tags.go`:

```go
type AddCaseTagsParams struct {
	TagIds []string `json:"tag_ids" binding:"required,min=1,dive,uuid"`
}

func HandleAddCaseTags(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := types.UuidParam(c, "caseId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params AddCaseTagsParams
		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()
		userUsecase := uc.NewUserUseCase()
		tagUsecase := uc.NewTagUseCase()

		cas, err := caseUsecase.AddCaseTags(ctx, caseId.String(), params.TagIds)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		users, err := userUsecase.ListUsers(ctx, &orgId)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		tags, err := tagUsecase.ListAllTags(ctx, orgId, models.TagTargetCase, false)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		referents, err := caseUsecase.GetCasesReferents(ctx, []string{cas.Id})
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.AdaptCase(users, tags, referents)(cas)).Serve(c)
	}
}
```

**Step 2: Add the remove-tag handler to `pubapi/v1/tags.go`**

Append to `pubapi/v1/tags.go`:

```go
func HandleRemoveCaseTag(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := types.UuidParam(c, "caseId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		tagId, err := types.UuidParam(c, "tagId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()

		if err := caseUsecase.RemoveCaseTag(ctx, caseId.String(), tagId.String()); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
```

You'll need to add `"net/http"` to the imports.

**Step 3: Register the routes in `BetaRoutes`**

In `pubapi/v1/routes.go`, add inside the `BetaRoutes` function, in the `root` group:

```go
root.POST("/cases/:caseId/tags", HandleAddCaseTags(uc))
root.DELETE("/cases/:caseId/tags/:tagId", HandleRemoveCaseTag(uc))
```

**Step 4: Verify the project compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 5: Commit**

```
feat: add POST and DELETE /v1beta/cases/:caseId/tags endpoints
```

---

### Task 7: Update mock and fix compilation

**Files:**
- Modify: Any mock files that implement the changed interfaces

**Step 1: Find and update mocks**

Search for mocks implementing `TagUseCaseRepository` or `CaseUseCaseRepository`:

```bash
grep -r "ListOrganizationTags\|GetCaseByIdForUpdate" mocks/
```

Update any mock implementations to match the new signatures. If mocks are auto-generated (via mockery), regenerate them:

```bash
go generate ./...
```

**Step 2: Verify the project compiles and tests pass**

Run: `go build ./...`
Run: `go test ./usecases/... ./repositories/... ./pubapi/...`
Expected: SUCCESS

**Step 3: Commit**

```
chore: update mocks for new tag and case repository interfaces
```

---

### Task 8: Update OpenAPI spec

**Files:**
- Modify: `pubapi/openapi/v1beta.yml`

**Step 1: Add the three new endpoint definitions to the v1beta OpenAPI spec**

Add the tag listing endpoint, add-case-tags endpoint, and remove-case-tag endpoint to the spec following the existing patterns. Include:
- Path definitions with parameters
- Request/response schemas
- Security requirements (BearerTokenAuth, ApiKeyAuth)
- Tag grouping (use a new "Tags" tag)

**Step 2: Commit**

```
docs: add case tags endpoints to v1beta OpenAPI spec
```

---

### Task 9: Manual integration test

**Step 1: Run the server locally**

```bash
go run . --migrations --server
```

**Step 2: Test the three endpoints manually using curl**

1. `GET /v1beta/tags` — verify paginated response
2. `GET /v1beta/tags?target=case` — verify filtered response
3. `POST /v1beta/cases/:caseId/tags` with `{"tag_ids": ["..."]}` — verify case updated
4. `DELETE /v1beta/cases/:caseId/tags/:tagId` — verify 204 response
5. Verify idempotency: add same tag twice, remove absent tag

**Step 3: Verify edge cases**

- Adding an object tag to a case returns 400
- Adding a non-existent tag returns 404
- Pagination `after` cursor works correctly
