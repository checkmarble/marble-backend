package api

import (
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

var casesPaginationDefaults = dto.PaginationDefaults{
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

		var paginationAndSorting dto.PaginationAndSortingInput
		if err := c.ShouldBind(&paginationAndSorting); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		paginationAndSorting = dto.WithPaginationDefaults(paginationAndSorting, casesPaginationDefaults)

		usecase := usecasesWithCreds(ctx, uc).NewCaseUseCase()
		cases, err := usecase.ListCases(c.Request.Context(), organizationId,
			dto.AdaptPaginationAndSortingInput(paginationAndSorting), filters)
		if presentError(ctx, c, err) {
			return
		}

		if len(cases) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"total_count": dto.AdaptTotalCount(models.TotalCount{}),
				"start_index": 0,
				"end_index":   0,
				"items":       []dto.APICase{},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"total_count": dto.AdaptTotalCount(cases[0].TotalCount),
			"start_index": cases[0].RankNumber,
			"end_index":   cases[len(cases)-1].RankNumber,
			"items":       pure_utils.Map(cases, func(c models.CaseWithRank) dto.APICase { return dto.AdaptCaseDto(c.Case) }),
		})
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
		inboxCase, err := usecase.GetCase(c.Request.Context(), caseInput.Id)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptCaseWithDecisionsDto(inboxCase))
	}
}

func handlePostCase(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(c.Request.Context())
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
			c.Request.Context(),
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
		creds, found := utils.CredentialsFromCtx(c.Request.Context())
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
		inboxCase, err := usecase.UpdateCase(c.Request.Context(), userId, models.UpdateCaseAttributes{
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

func handlePostCaseDecisions(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(c.Request.Context())
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
		inboxCase, err := usecase.AddDecisionsToCase(c.Request.Context(), userId, caseInput.Id, data.DecisionIds)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"case": dto.AdaptCaseWithDecisionsDto(inboxCase)})
	}
}

func handlePostCaseComment(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(c.Request.Context())
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
		inboxCase, err := usecase.CreateCaseComment(c.Request.Context(), userId, models.CreateCaseCommentAttributes{
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
		creds, found := utils.CredentialsFromCtx(c.Request.Context())
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
		inboxCase, err := usecase.CreateCaseTags(c.Request.Context(), userId, models.CreateCaseTagsAttributes{
			CaseId: caseInput.Id,
			TagIds: data.TagIds,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{"case": dto.AdaptCaseWithDecisionsDto(inboxCase)})
	}
}

type FileForm struct {
	File *multipart.FileHeader `form:"file" binding:"required"`
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
		cs, err := usecase.CreateCaseFile(c.Request.Context(), models.CreateCaseFileInput{
			CaseId: caseInput.Id,
			File:   form.File,
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
		url, err := usecase.GetCaseFileUrl(c.Request.Context(), caseFileInput.Id)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}

func handleReviewCaseDecisions(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(c.Request.Context())
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
		case_, err := usecase.ReviewCaseDecisions(c.Request.Context(),
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
