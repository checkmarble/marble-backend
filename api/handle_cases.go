package api

import (
	"cmp"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/ai_agent"
	"github.com/checkmarble/marble-backend/utils"
)

var casesPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.CasesSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func handleListCases(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var filters dto.CaseFilters
		if err := c.ShouldBind(&filters); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		parsedFilters, err := filters.Parse()
		if presentError(ctx, c, err) {
			return
		}

		var paginationAndSortingDto dto.PaginationAndSorting
		if err := c.ShouldBind(&paginationAndSortingDto); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		paginationAndSorting := models.WithPaginationDefaults(
			dto.AdaptPaginationAndSorting(paginationAndSortingDto), casesPaginationDefaults)

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		cases, err := usecase.ListCases(ctx, organizationId, paginationAndSorting, parsedFilters)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptCaseListPage(cases))
	}
}

type CaseInput struct {
	Id string `uri:"case_id" binding:"required,uuid"`
}

func handleGetCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var caseInput CaseInput
		if err := c.ShouldBindUri(&caseInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		inboxCase, err := usecase.GetCase(ctx, caseInput.Id)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptCaseWithDetailsDto(inboxCase))
	}
}

func handlePostCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}
		userId := string(creds.ActorIdentity.UserId)

		var data dto.CreateCaseBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		inboxCase, err := usecase.CreateCaseAsUser(
			ctx,
			organizationId,
			userId,
			models.CreateCaseAttributes{
				DecisionIds:    data.DecisionIds,
				InboxId:        data.InboxId,
				Name:           data.Name,
				OrganizationId: organizationId,
				AssigneeId:     &userId,
				Type:           models.CaseTypeDecision, // By default, we can only create cases from decisions
			})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"case": dto.AdaptCaseWithDetailsDto(inboxCase),
		})
	}
}

func handlePatchCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}
		userId := string(creds.ActorIdentity.UserId)

		var caseInput CaseInput
		if err := c.ShouldBindUri(&caseInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var data dto.UpdateCaseBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		inboxCase, err := usecase.UpdateCase(ctx, userId, models.UpdateCaseAttributes{
			Id:      caseInput.Id,
			Name:    data.Name,
			Status:  models.CaseStatus(data.Status),
			Outcome: models.CaseOutcome(data.Outcome),
			InboxId: data.InboxId,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"case": dto.AdaptCaseWithDetailsDto(inboxCase),
		})
	}
}

type CaseSnoozeParams struct {
	Until time.Time `json:"until"`
}

func handleSnoozeCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		userId := creds.ActorIdentity.UserId
		caseId := c.Param("case_id")

		var params CaseSnoozeParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			presentError(ctx, c, err)
			return
		}

		if params.Until.Before(time.Now()) {
			presentError(ctx, c, errors.Wrap(models.BadParameterError,
				"a case cannot only be snoozed until a future date"))
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()

		req := models.CaseSnoozeRequest{
			UserId: userId,
			CaseId: caseId,
			Until:  params.Until,
		}

		if err := caseUsecase.Snooze(ctx, req); err != nil {
			presentError(ctx, c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleUnsnoozeCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		userId := creds.ActorIdentity.UserId
		caseId := c.Param("case_id")

		uc := usecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()

		req := models.CaseSnoozeRequest{
			UserId: userId,
			CaseId: caseId,
		}

		if err := caseUsecase.Unsnooze(ctx, req); err != nil {
			presentError(ctx, c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleListCaseDecisions(uc usecases.Usecases, marbleAppUrl *url.URL) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, _ := utils.OrganizationIdFromRequest(c.Request)
		caseId := c.Param("case_id")
		cursorId := c.Query("cursor_id")
		limit := models.CaseDecisionsPerPage

		if c.Query("limit") != "" {
			l, err := strconv.Atoi(c.Query("limit"))
			if err != nil {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
				return
			}

			limit = l
		}

		req := models.CaseDecisionsRequest{
			OrgId:    orgId,
			CaseId:   caseId,
			CursorId: cursorId,
			Limit:    limit,
		}

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		decisions, hasMore, err := usecase.ListCaseDecisions(ctx, req)

		if presentError(ctx, c, err) {
			return
		}

		nextCursorId := ""

		if len(decisions) > 0 {
			nextCursorId = decisions[len(decisions)-1].DecisionId.String()
		}

		c.JSON(http.StatusOK, dto.CaseDecisionListDto{
			Decisions: pure_utils.Map(decisions, func(d models.DecisionWithRulesAndScreeningsBaseInfo) dto.DecisionWithRules {
				return dto.NewDecisionWithRuleBaseInfoDto(d, marbleAppUrl)
			}),
			Pagination: dto.CaseDecisionListPaginationDto{
				HasMore:      hasMore,
				NextCursorId: nextCursorId,
			},
		})
	}
}

func handlePostCaseDecisions(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}
		userId := string(creds.ActorIdentity.UserId)

		var caseInput CaseInput
		if err := c.ShouldBindUri(&caseInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var data dto.AddDecisionToCaseBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		inboxCase, err := usecase.AddDecisionsToCase(ctx, userId, caseInput.Id, data.DecisionIds)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"case": dto.AdaptCaseWithDetailsDto(inboxCase)})
	}
}

func handlePostCaseComment(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}
		userId := string(creds.ActorIdentity.UserId)

		var caseInput CaseInput
		if err := c.ShouldBindUri(&caseInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var data dto.CreateCaseCommentBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		inboxCase, err := usecase.CreateCaseComment(ctx, userId, models.CreateCaseCommentAttributes{
			Id:      caseInput.Id,
			Comment: data.Comment,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"case": dto.AdaptCaseWithDetailsDto(inboxCase),
		})
	}
}

func handlePostCaseTags(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}
		userId := string(creds.ActorIdentity.UserId)

		var caseInput CaseInput
		if err := c.ShouldBindUri(&caseInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var data dto.CreateCaseTagBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		inboxCase, err := usecase.CreateCaseTags(ctx, userId, models.CreateCaseTagsAttributes{
			CaseId: caseInput.Id,
			TagIds: data.TagIds,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{"case": dto.AdaptCaseWithDetailsDto(inboxCase)})
	}
}

func handleAssignCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caseId := c.Param("case_id")
		creds, _ := utils.CredentialsFromCtx(ctx)

		var payload dto.CaseAssigneeDto

		if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
			presentError(ctx, c, err)
			return
		}

		req := models.CaseAssignementRequest{
			UserId:     creds.ActorIdentity.UserId,
			CaseId:     caseId,
			AssigneeId: &payload.UserId,
		}

		if payload.UserId == "me" {
			req.AssigneeId = &creds.ActorIdentity.UserId
		}

		uc := usecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()

		if err := caseUsecase.AssignCase(ctx, req); err != nil {
			presentError(ctx, c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleUnassignCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caseId := c.Param("case_id")
		creds, _ := utils.CredentialsFromCtx(ctx)

		req := models.CaseAssignementRequest{
			UserId: creds.ActorIdentity.UserId,
			CaseId: caseId,
		}

		uc := usecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()

		if err := caseUsecase.UnassignCase(ctx, req); err != nil {
			presentError(ctx, c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

type FileForm struct {
	Files []multipart.FileHeader `form:"file[]" binding:"required"`
}

func handlePostCaseFile(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var caseInput CaseInput
		if err := c.ShouldBindUri(&caseInput); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var form FileForm
		if err := c.ShouldBind(&form); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		cs, _, err := usecase.CreateCaseFiles(ctx, models.CreateCaseFilesInput{
			CaseId: caseInput.Id,
			Files:  form.Files,
		})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, gin.H{"case": dto.AdaptCaseWithDetailsDto(cs)})
	}
}

type CaseFileInput struct {
	Id string `uri:"case_file_id" binding:"required,uuid"`
}

func handleDownloadCaseFile(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var caseFileInput CaseFileInput
		if err := c.ShouldBindUri(&caseFileInput); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		url, err := usecase.GetCaseFileUrl(ctx, caseFileInput.Id)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}

func handleReviewCaseDecisions(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}
		userId := string(creds.ActorIdentity.UserId)

		var data dto.ReviewCaseDecisionsBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		case_, err := usecase.ReviewCaseDecisions(ctx,
			models.ReviewCaseDecisionsBody{
				DecisionId:    data.DecisionId,
				ReviewComment: data.ReviewComment,
				ReviewStatus:  data.ReviewStatus,
				UserId:        userId,
			})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"case": dto.AdaptCaseWithDetailsDto(case_)})
	}
}

func handleGetRelatedCasesByPivotValue(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		pivotValue := c.Param("pivotValue")

		uc := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		cases, err := uc.GetRelatedCasesByPivotValue(ctx, creds.OrganizationId.String(), pivotValue)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(cases, dto.AdaptCaseDto))
	}
}

func handleGetRelatedContinuousScreeningCasesByObjectAttr(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		objectType := c.Param("objectType")
		if objectType == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "objectType is required"))
			return
		}
		objectId := c.Param("objectId")
		if objectId == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "objectId is required"))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		cases, err := uc.GetRelatedContinuousScreeningCasesByObjectAttr(ctx,
			creds.OrganizationId.String(), objectType, objectId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(cases, dto.AdaptCaseDto))
	}
}

func handleReadCasePivotObjects(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caseId := c.Param("case_id")

		uc := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		pivotObjects, err := uc.ReadCasePivotObjects(ctx, caseId)
		if presentError(ctx, c, err) {
			return
		}

		// Sort them for idempotent behavior, in the case the frontend makes assumptions on the order
		slices.SortStableFunc(pivotObjects, func(a, b models.PivotObject) int {
			return cmp.Or(
				strings.Compare(a.PivotId, b.PivotId),
				strings.Compare(a.PivotValue, b.PivotValue),
			)
		})
		pivotObjectDtos, err := pure_utils.MapErr(pivotObjects, dto.AdaptPivotObjectDto)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"pivot_objects": pivotObjectDtos})
	}
}

func handleEscalateCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caseId := c.Param("case_id")

		uc := usecasesWithCreds(ctx, uc).NewCaseUseCase()

		if err := uc.EscalateCase(ctx, caseId); err != nil {
			presentError(ctx, c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleGetNextCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		caseId := c.Param("case_id")

		uc := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		nextCaseId, err := uc.GetNextCaseId(ctx, orgId, caseId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"id": nextCaseId})
	}
}

func handleGetCaseDataForCopilot(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caseId := c.Param("case_id")

		uc := usecasesWithCreds(ctx, uc).NewAiAgentUsecase()

		zipReader, err := uc.GetCaseDataZip(ctx, caseId)
		if presentError(ctx, c, err) {
			return
		}

		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", "attachment; filename=test.zip")
		if _, err := io.Copy(c.Writer, zipReader); err != nil {
			presentError(ctx, c, err)
			return
		}
		c.Status(http.StatusOK)
	}
}

func handleGetCaseReview(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caseId, err := uuid.Parse(c.Param("case_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewAiAgentUsecase()
		reviews, err := usecase.GetCaseReview(ctx, caseId.String())
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, reviews)
	}
}

func handleEnqueueCaseReview(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caseId, err := uuid.Parse(c.Param("case_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewAiAgentUsecase()
		ok, err := usecase.EnqueueCreateCaseReview(ctx, caseId.String())
		if !ok {
			presentError(ctx, c, errors.Wrap(models.ForbiddenError, "AI case review is not enabled"))
			return
		}
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handlePutCaseReviewFeedback(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caseId := c.Param("case_id")
		reviewId, err := uuid.Parse(c.Param("review_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var feedback agent_dto.UpdateCaseReviewFeedbackDto
		if err := c.ShouldBindJSON(&feedback); presentError(ctx, c, err) {
			return
		}
		if err := feedback.Validate(); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewAiAgentUsecase()
		review, err := usecase.UpdateAiCaseReviewFeedback(ctx, caseId, reviewId, feedback.Adapt())
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, review)
	}
}

func handleEnrichCasePivotObjects(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		caseId := c.Param("case_id")
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewAiAgentUsecase()
		responses, err := usecase.EnrichCasePivotObjects(ctx, orgId, caseId)
		if err != nil {
			if errors.Is(err, ai_agent.ErrKYCEnrichmentNotEnabled) {
				presentError(ctx, c, errors.Wrap(models.ForbiddenError, "KYC enrichment is not enabled"))
				return
			}
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusOK, agent_dto.AdaptKYCEnrichmentResultsDto(responses))
	}
}

func handleCaseMassUpdate(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var params dto.CaseMassUpdateDto

		if err := c.ShouldBindJSON(&params); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewCaseUseCase()

		if err := uc.MassUpdate(ctx, params); presentError(ctx, c, err) {
			return
		}
	}
}
