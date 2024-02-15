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
	"github.com/checkmarble/marble-backend/utils"
)

var casesPaginationDefaults = dto.PaginationDefaults{
	Limit:  25,
	SortBy: models.CasesSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func (api *API) handleListCases(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
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

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	cases, err := usecase.ListCases(c.Request.Context(), organizationId,
		dto.AdaptPaginationAndSortingInput(paginationAndSorting), filters)
	if presentError(c, err) {
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

type CaseInput struct {
	Id string `uri:"case_id" binding:"required,uuid"`
}

func (api *API) handleGetCase(c *gin.Context) {
	var caseInput CaseInput
	if err := c.ShouldBindUri(&caseInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	inboxCase, err := usecase.GetCase(c.Request.Context(), caseInput.Id)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, dto.AdaptCaseWithDecisionsDto(inboxCase))
}

func (api *API) handlePostCase(c *gin.Context) {
	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
		return
	}
	userId := string(creds.ActorIdentity.UserId)

	var data dto.CreateCaseBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	inboxCase, err := usecase.CreateCase(c.Request.Context(), userId, models.CreateCaseAttributes{
		DecisionIds:    data.DecisionIds,
		InboxId:        data.InboxId,
		Name:           data.Name,
		OrganizationId: organizationId,
	})

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"case": dto.AdaptCaseWithDecisionsDto(inboxCase),
	})
}

func (api *API) handlePatchCase(c *gin.Context) {
	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
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

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	inboxCase, err := usecase.UpdateCase(c.Request.Context(), userId, models.UpdateCaseAttributes{
		Id:      caseInput.Id,
		Name:    data.Name,
		Status:  models.CaseStatus(data.Status),
		InboxId: data.InboxId,
	})

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"case": dto.AdaptCaseWithDecisionsDto(inboxCase),
	})
}

func (api *API) handlePostCaseDecisions(c *gin.Context) {
	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
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

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	inboxCase, err := usecase.AddDecisionsToCase(c.Request.Context(), userId, caseInput.Id, data.DecisionIds)

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"case": dto.AdaptCaseWithDecisionsDto(inboxCase)})
}

func (api *API) handlePostCaseComment(c *gin.Context) {
	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
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

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	inboxCase, err := usecase.CreateCaseComment(c.Request.Context(), userId, models.CreateCaseCommentAttributes{
		Id:      caseInput.Id,
		Comment: data.Comment,
	})

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"case": dto.AdaptCaseWithDecisionsDto(inboxCase),
	})
}

func (api *API) handlePostCaseTags(c *gin.Context) {
	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
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

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	inboxCase, err := usecase.CreateCaseTags(c.Request.Context(), userId, models.CreateCaseTagsAttributes{
		CaseId: caseInput.Id,
		TagIds: data.TagIds,
	})

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{"case": dto.AdaptCaseWithDecisionsDto(inboxCase)})
}

type FileForm struct {
	File *multipart.FileHeader `form:"file" binding:"required"`
}

func (api *API) handlePostCaseFile(c *gin.Context) {
	var caseInput CaseInput
	if err := c.ShouldBindUri(&caseInput); err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}

	var form FileForm
	if err := c.ShouldBind(&form); err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	cs, err := usecase.CreateCaseFile(c.Request.Context(), models.CreateCaseFileInput{
		CaseId: caseInput.Id,
		File:   form.File,
	})
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusCreated, gin.H{"case": dto.AdaptCaseWithDecisionsDto(cs)})
}

type CaseFileInput struct {
	Id string `uri:"case_file_id" binding:"required,uuid"`
}

func (api *API) handleDownloadCaseFile(c *gin.Context) {
	var caseFileInput CaseFileInput
	if err := c.ShouldBindUri(&caseFileInput); err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	url, err := usecase.GetCaseFileUrl(c.Request.Context(), caseFileInput.Id)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}
