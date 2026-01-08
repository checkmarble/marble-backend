package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func handleListSuspiciousActivityReports(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		caseId := c.Param("case_id")

		uc := usecasesWithCreds(ctx, uc)
		sarUsecase := uc.NewSuspiciousActivityReportUsecase()

		sars, err := sarUsecase.ListReports(ctx, creds.OrganizationId, caseId)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusCreated, pure_utils.Map(sars, dto.AdaptSuspiciousActivityReportDto))
	}
}

// multipart endpoint to accept file uploads at the same time as status
func handleCreateSuspiciousActivityReport(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		var params dto.SuspiciousActivityReportParams

		caseId := c.Param("case_id")

		if err := c.ShouldBind(&params); err != nil {
			presentError(ctx, c, err)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		sarUsecase := uc.NewSuspiciousActivityReportUsecase()

		var status *models.SarStatus

		if params.Status != nil {
			status = utils.Ptr(models.SarStatusFromString(*params.Status))
		}

		req := models.SuspiciousActivityReportRequest{
			CaseId:    caseId,
			Status:    status,
			File:      params.File,
			CreatedBy: creds.ActorIdentity.UserId,
		}

		if params.File != nil {
			req.UploadedBy = &creds.ActorIdentity.UserId
		}

		sar, err := sarUsecase.CreateReport(ctx, creds.OrganizationId, req)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptSuspiciousActivityReportDto(sar))
	}
}

// multipart endpoint to accept file uploads at the same time as status
func handleUpdateSuspiciousActivityReport(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		var params dto.SuspiciousActivityReportParams

		caseId := c.Param("case_id")
		reportId := c.Param("reportId")

		if err := c.ShouldBind(&params); err != nil {
			presentError(ctx, c, err)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		sarUsecase := uc.NewSuspiciousActivityReportUsecase()

		var status *models.SarStatus

		if params.Status != nil {
			status = utils.Ptr(models.SarStatusFromString(*params.Status))
		}

		req := models.SuspiciousActivityReportRequest{
			CaseId:   caseId,
			ReportId: &reportId,
			Status:   status,
			File:     params.File,
		}

		if params.File != nil {
			req.UploadedBy = &creds.ActorIdentity.UserId
		}

		sar, err := sarUsecase.UpdateReport(ctx, creds.OrganizationId, req)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusOK, dto.AdaptSuspiciousActivityReportDto(sar))
	}
}

func handleDownloadFileToSuspiciousActivityReport(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		caseId := c.Param("case_id")
		reportId := c.Param("reportId")

		uc := usecasesWithCreds(ctx, uc)
		sarUsecase := uc.NewSuspiciousActivityReportUsecase()

		reportUrl, err := sarUsecase.GenerateReportUrl(ctx, creds.OrganizationId, caseId, reportId)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": reportUrl})
	}
}

func handleDeleteSuspiciousActivityReport(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		caseId := c.Param("case_id")
		reportId := c.Param("reportId")

		uc := usecasesWithCreds(ctx, uc)
		sarUsecase := uc.NewSuspiciousActivityReportUsecase()

		req := models.SuspiciousActivityReportRequest{
			CaseId:   caseId,
			ReportId: &reportId,
		}

		if err := sarUsecase.DeleteReport(ctx, creds.OrganizationId, req); err != nil {
			presentError(ctx, c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
