package v1

import (
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var casePaginationDefaults = models.PaginationDefaults{
	Limit:  50,
	SortBy: models.CasesSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func HandleListCases(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var p params.ListCasesParams

		if err := c.ShouldBindQuery(&p); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		if !p.StartDate.IsZero() && !p.EndDate.IsZero() {
			if time.Time(p.StartDate).After(time.Time(p.EndDate)) {
				pubapi.NewErrorResponse().WithError(errors.WithDetail(
					pubapi.ErrInvalidPayload, "end date should be after start date")).Serve(c)
				return
			}
		}

		filters, err := p.ToFilters().Parse()
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		filters.UseLinearOrdering = true

		paging := p.PaginationParams.ToModel(casePaginationDefaults)

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()
		userUsecase := uc.NewUserUseCase()
		tagUsecase := uc.NewTagUseCase()

		cases, err := caseUsecase.ListCases(ctx, orgId, paging, filters)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		caseIds := pure_utils.Map(cases.Cases, func(cas models.Case) string { return cas.Id })

		users, err := userUsecase.ListUsers(ctx, &orgId)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		referents, err := caseUsecase.GetCasesReferents(ctx, caseIds)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		tags, err := tagUsecase.ListAllTags(ctx, orgId, models.TagTargetCase, false)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		nextPageId := ""

		if len(cases.Cases) > 0 {
			nextPageId = cases.Cases[len(cases.Cases)-1].Id
		}

		pubapi.
			NewResponse(pure_utils.Map(cases.Cases, dto.AdaptCase(users, tags, referents))).
			WithPagination(cases.HasNextPage, nextPageId).
			Serve(c)
	}
}

func HandleGetCase(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := pubapi.UuidParam(c, "caseId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()
		userUsecase := uc.NewUserUseCase()
		tagUsecase := uc.NewTagUseCase()

		cas, err := caseUsecase.GetCase(ctx, caseId.String())
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		users, err := userUsecase.ListUsers(ctx, &orgId)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		referents, err := caseUsecase.GetCasesReferents(ctx, []string{cas.Id})
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		tags, err := tagUsecase.ListAllTags(ctx, orgId, models.TagTargetCase, false)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(dto.AdaptCase(users, tags, referents)(cas)).
			Serve(c)
	}
}

type CreateCaseParams struct {
	Inbox     uuid.UUID   `json:"inbox" binding:"required"`
	Name      string      `json:"name" binding:"required"`
	Decisions []uuid.UUID `json:"decisions"`
	Assignee  string      `json:"assignee"`
}

func HandleCreateCase(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var params CreateCaseParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		caseUsecase := uc.NewCaseUseCase()
		userUsecase := uc.NewUserUseCase()

		req := models.CreateCaseAttributes{
			OrganizationId: orgId,
			InboxId:        params.Inbox,
			Name:           params.Name,
			DecisionIds:    pure_utils.Map(params.Decisions, func(id uuid.UUID) string { return id.String() }),
		}

		if params.Assignee != "" {
			user, err := userUsecase.GetUserByEmail(ctx, params.Assignee)
			if err != nil {
				pubapi.NewErrorResponse().WithError(err).Serve(c)
				return
			}

			req.AssigneeId = utils.Ptr(string(user.UserId))
		}

		cas, err := caseUsecase.CreateCaseAsApiClient(ctx, orgId, req)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		referents, err := caseUsecase.GetCasesReferents(ctx, []string{cas.Id})
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.NewResponse(dto.AdaptCase(nil, nil, referents)(cas)).Serve(c)
	}
}

type UpdateCaseParams struct {
	Inbox uuid.UUID `json:"inbox"`
	Name  string    `json:"name"`
}

func HandleUpdateCase(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := pubapi.UuidParam(c, "caseId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params UpdateCaseParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		caseUsecase := uc.NewCaseUseCase()

		req := models.UpdateCaseAttributes{
			Id: caseId.String(),
		}

		if params.Inbox != uuid.Nil {
			req.InboxId = &params.Inbox
		}
		if params.Name != "" {
			req.Name = params.Name
		}

		cas, err := caseUsecase.UpdateCase(ctx, "", req)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		referents, err := caseUsecase.GetCasesReferents(ctx, []string{cas.Id})
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.NewResponse(dto.AdaptCase(nil, nil, referents)(cas)).Serve(c)
	}
}

type CloseCaseParams struct {
	Outcome string `json:"outcome" binding:"omitempty,oneof=unset confirmed_risk valuable_alert false_positive"`
}

func HandleSetCaseStatus(uc usecases.Usecases, status models.CaseStatus) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := pubapi.UuidParam(c, "caseId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		caseUsecase := uc.NewCaseUseCase()

		req := models.UpdateCaseAttributes{
			Id:     caseId.String(),
			Status: status,
		}

		if status == models.CaseClosed {
			var params CloseCaseParams

			if err := c.ShouldBindBodyWithJSON(&params); err != nil {
				if !errors.Is(err, io.EOF) {
					pubapi.NewErrorResponse().WithError(err).Serve(c)
					return
				}
			}

			if params.Outcome != "" {
				req.Outcome = models.CaseOutcome(params.Outcome)
			}
		}

		cas, err := caseUsecase.UpdateCase(ctx, "", req)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		referents, err := caseUsecase.GetCasesReferents(ctx, []string{cas.Id})
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.NewResponse(dto.AdaptCase(nil, nil, referents)(cas)).Serve(c)
	}
}

func HandleEscalateCase(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := pubapi.UuidParam(c, "caseId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		caseUsecase := uc.NewCaseUseCase()

		err = caseUsecase.EscalateCase(ctx, caseId.String())
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		cas, err := caseUsecase.GetCase(ctx, caseId.String())
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		referents, err := caseUsecase.GetCasesReferents(ctx, []string{cas.Id})
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.NewResponse(dto.AdaptCase(nil, nil, referents)(cas)).Serve(c)
	}
}

func HandleListCaseComments(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		caseId, err := pubapi.UuidParam(c, "caseId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var p pubapi.PaginationParams

		if err := c.ShouldBindQuery(&p); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		paging := p.ToModel(casePaginationDefaults)
		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()
		userUsecase := uc.NewUserUseCase()

		users, err := userUsecase.ListUsers(ctx, &orgId)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		comments, err := caseUsecase.GetCaseComments(ctx, caseId.String(), paging)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		nextPageId := ""

		if len(comments.Items) > 0 {
			nextPageId = comments.Items[len(comments.Items)-1].Id
		}

		pubapi.
			NewResponse(pure_utils.Map(comments.Items, dto.AdaptCaseComment(users))).
			WithPagination(comments.HasNextPage, nextPageId).
			Serve(c)
	}
}

type CreateCommentParams struct {
	Comment string `json:"comment"`
}

func HandleCreateComment(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := pubapi.UuidParam(c, "caseId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params CreateCommentParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		caseUsecase := uc.NewCaseUseCase()

		req := models.CreateCaseCommentAttributes{
			Id:      caseId.String(),
			Comment: params.Comment,
		}

		if _, err := caseUsecase.CreateCaseComment(ctx, "", req); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		c.Status(http.StatusCreated)
	}
}

func HandleListCaseFiles(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := pubapi.UuidParam(c, "caseId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()

		files, err := caseUsecase.GetCaseFiles(ctx, caseId.String())
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(pure_utils.Map(files, dto.AdaptCaseFile)).
			Serve(c)
	}
}

func HandleDownloadCaseFile(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		fileId, err := pubapi.UuidParam(c, "fileId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()

		url, err := caseUsecase.GetCaseFileUrl(ctx, fileId.String())
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.Redirect(c, url)
	}
}

type FileForm struct {
	Files []multipart.FileHeader `form:"file" binding:"required"`
}

func HandleCreateCaseFile(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := pubapi.UuidParam(c, "caseId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var form FileForm

		if err := c.ShouldBind(&form); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()

		req := models.CreateCaseFilesInput{
			CaseId: caseId.String(),
			Files:  form.Files,
		}

		_, files, err := caseUsecase.CreateCaseFiles(ctx, req)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(pure_utils.Map(files, dto.AdaptCaseFile)).
			Serve(c)
	}
}
