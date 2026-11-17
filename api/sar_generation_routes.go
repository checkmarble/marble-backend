package api

import (
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/usecases"
)

// addSarGenerationRoutes registers the SAR declaration routes (flat paths, all
// with the timeout middleware). Also used by the handler tests.
func addSarGenerationRoutes(router gin.IRoutes, tom gin.HandlerFunc, uc usecases.Usecases) {
	router.GET("/sar-formats", tom, handleListSarFormats(uc))
	router.GET("/sar-formats/:format_key/spec", tom, handleGetSarFormatSpec(uc))
	router.GET("/sar-format-settings/:family", tom, handleGetSarOrgSettings(uc))
	router.PUT("/sar-format-settings/:family", tom, handlePutSarOrgSettings(uc))
	router.POST("/cases/:case_id/sar-declarations", tom, handleCreateSarDeclaration(uc))
	router.GET("/cases/:case_id/sar-declarations", tom, handleListSarDeclarations(uc))
	router.GET("/sar-declarations/:id", tom, handleGetSarDeclaration(uc))
	router.PATCH("/sar-declarations/:id/fields", tom, handleUpdateSarFields(uc))
	router.POST("/sar-declarations/:id/items", tom, handleAddSarItem(uc))
	router.DELETE("/sar-declarations/:id/items", tom, handleRemoveSarItem(uc))
	router.POST("/sar-declarations/:id/mark-reviewed", tom, handleMarkSarReviewed(uc))
	router.POST("/sar-declarations/:id/exports", tom, handleExportSarDeclaration(uc))
	router.GET("/sar-declarations/:id/exports", tom, handleListSarExports(uc))
	router.GET("/sar-declarations/:id/exports/:export_id/download", tom, handleDownloadSarExport(uc))
	router.PUT("/sar-declarations/:id/analysis", tom, handleUpdateSarAnalysis(uc))
	router.GET("/sar-declarations/:id/candidates", tom, handleListSarCandidates(uc))
	router.GET("/sar-declarations/:id/navigate", tom, handleNavigateSar(uc))
	router.GET("/sar-declarations/:id/selection", tom, handleGetSarSelection(uc))
	router.PATCH("/sar-declarations/:id/selection", tom, handleUpdateSarSelection(uc))
	router.POST("/sar-declarations/:id/generate", tom, handleGenerateSar(uc))
	router.GET("/sar-generation-jobs/:job_id", tom, handleGetSarJob(uc))
	router.POST("/sar-generation-jobs/:job_id/resume", tom, handleResumeSarJob(uc))
	router.POST("/sar-declarations/:id/review-check", tom, handleStartSarReview(uc))
	router.GET("/sar-declarations/:id/questions", tom, handleListSarQuestions(uc))
	router.POST("/sar-questions/:question_id/choose", tom, handleChooseSarQuestionOption(uc))
	router.POST("/sar-questions/:question_id/dismiss", tom, handleDismissSarQuestion(uc))
}
