package api

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
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
			c.Status(http.StatusBadRequest)
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
		cases, err := usecase.ListCases(ctx, organizationId, paginationAndSorting, filters)
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

		c.JSON(http.StatusOK, dto.AdaptCaseWithDecisionsDto(inboxCase))
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
			})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"case": dto.AdaptCaseWithDecisionsDto(inboxCase),
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
			InboxId: data.InboxId,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"case": dto.AdaptCaseWithDecisionsDto(inboxCase),
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
		c.JSON(http.StatusOK, gin.H{"case": dto.AdaptCaseWithDecisionsDto(inboxCase)})
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
			"case": dto.AdaptCaseWithDecisionsDto(inboxCase),
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
		c.JSON(http.StatusCreated, gin.H{"case": dto.AdaptCaseWithDecisionsDto(inboxCase)})
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
		cs, err := usecase.CreateCaseFiles(ctx, models.CreateCaseFilesInput{
			CaseId: caseInput.Id,
			Files:  form.Files,
		})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, gin.H{"case": dto.AdaptCaseWithDecisionsDto(cs)})
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
		c.JSON(http.StatusOK, gin.H{"case": dto.AdaptCaseWithDecisionsDto(case_)})
	}
}

func handleGetRelatedCases(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		decisionId := c.Param("decision_id")

		uc := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		cases, err := uc.GetRelatedCases(ctx, creds.OrganizationId, decisionId)
		if err != nil {
			presentError(ctx, c, err)
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
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		// Sort them for idempotent behavior, in the case the frontend makes assumptions on the order
		sort.Slice(pivotObjects, func(i, j int) bool {
			return (pivotObjects[i].PivotId < pivotObjects[j].PivotId) ||
				(pivotObjects[i].PivotId == pivotObjects[j].PivotId &&
					pivotObjects[i].PivotValue < pivotObjects[j].PivotValue)
		})

		c.JSON(http.StatusOK, gin.H{"pivot_objects": pure_utils.Map(pivotObjects, dto.AdaptPivotObjectDto)})
	}
}
