package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"text/template"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/billing"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
)

// TODO: the current version of the code uses some static heuristics to limit the amount of data loaded into the context.
// A more advanced solution would be to compute this dynamically from the different elements that are passed. This is for a
// future iteration, because it is expected not to impact the P99 of case reviews.
const (
	HIGH_NB_ROWS_THRESHOLD                     = 100
	MAX_DECISIONS_REVIEW_PER_CASE              = 5
	MAX_PREVIOUS_CASES_REVIEW_PER_PIVOT_OBJECT = 10
	MAX_DECISIONS_FROM_PREVIOUS_CASES_REVIEW   = 50
)

var ReviewLevelEnum = []string{"probable_false_positive", "investigate", "escalate"}

// Constants for the case review prompt paths
const (
	PROMPT_CASE_REVIEW_PATH                          = "prompts/case_review/case_review.md"
	PROMPT_DATA_MODEL_OBJECT_FIELD_READ_OPTIONS_PATH = "prompts/case_review/data_model_object_field_read_options.md"
	PROMPT_DATA_MODEL_SUMMARY_PATH                   = "prompts/case_review/data_model_summary.md"
	PROMPT_RULE_DEFINITIONS_PATH                     = "prompts/case_review/rule_definitions.md"
	PROMPT_RULE_THRESHOLD_VALUES_PATH                = "prompts/case_review/rule_threshold_values.md"
	PROMPT_SANITY_CHECK_PATH                         = "prompts/case_review/sanity_check.md"
	INSTRUCTION_CUSTOM_REPORT_PATH                   = "prompts/case_review/instruction_custom_report.md"
	INSTRUCTION_LANGUAGE_PATH                        = "prompts/case_review/instruction_language.md"
	INSTRUCTION_STRUCTURE_PATH                       = "prompts/case_review/instruction_structure.md"
	SYSTEM_PROMPT_PATH                               = "prompts/system.md"
)

type sanityCheckOutput struct {
	Ok            bool    `json:"ok" jsonschema_description:"Whether the case review is ok or not" jsonschema:"required"`
	Justification string  `json:"justification" jsonschema_description:"Detailed justification for the sanity check, only in the case of a negative answer"`
	ReviewLevel   *string `json:"review_level" jsonschema_description:"Estimated review level, based on the case review that was received. Required if the sanity check is ok." jsonschema:"enum=probable_false_positive,enum=investigate,enum=escalate"`
}

// The jsonschema used for the case review output are set dynamically, because the list of enum values depends on the user.
type caseReviewOutput struct {
	CaseReview string  `json:"case_review"`
	Proofs     []proof `json:"proofs"`
}
type proof struct {
	Id     string               `json:"id"`
	Type   string               `json:"type"`
	Origin agent_dto.OriginName `json:"origin"`
	Reason string               `json:"reason"`
}

type customOrgInstructions struct {
	Language       *string `json:"language"`
	Structure      *string `json:"structure"`
	OrgDescription *string `json:"org_description"`
}

// Get from ai setting
// Not all organizations have custom instructions
func getOrganizationCustomInstructions(aiSetting models.AiSetting) customOrgInstructions {
	return customOrgInstructions{
		Language:       utils.Ptr(aiSetting.CaseReviewSetting.Language),
		Structure:      aiSetting.CaseReviewSetting.Structure,
		OrgDescription: aiSetting.CaseReviewSetting.OrgDescription,
	}
}

func (uc *AiAgentUsecase) getCaseReviewById(ctx context.Context, reviewId uuid.UUID) (agent_dto.AiCaseReviewOutputDto, error) {
	exec := uc.executorFactory.NewExecutor()
	review, err := uc.repository.GetCaseReviewById(ctx, exec, reviewId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{},
			errors.Wrap(err, "could not get case review by id")
	}

	blob, err := uc.blobRepository.GetBlob(ctx, review.BucketName, review.FileReference)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{},
			errors.Wrap(err, "could not get case review file")
	}
	defer blob.ReadCloser.Close()

	var reviewDto agent_dto.AiCaseReviewDto
	if review.Status == models.AiCaseReviewStatusCompleted {
		reviewDto, err = agent_dto.UnmarshalCaseReviewDto(
			review.DtoVersion, blob.ReadCloser)
		if err != nil {
			return agent_dto.AiCaseReviewOutputDto{},
				errors.Wrap(err, "could not unmarshal case review file")
		}
	}

	var reaction *string
	if review.Reaction != nil {
		reaction = utils.Ptr(review.Reaction.String())
	}

	return agent_dto.AiCaseReviewOutputDto{
		Id:       review.Id,
		Reaction: reaction,
		Version:  review.DtoVersion,
		Review:   reviewDto,
	}, nil
}

// returns a slice of 0 or 1 case review dto, the most recent one.
func (uc *AiAgentUsecase) getMostRecentCaseReview(ctx context.Context, caseId string) ([]agent_dto.AiCaseReviewOutputDto, error) {
	exec := uc.executorFactory.NewExecutor()
	_, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case with permissions")
	}

	caseIdUuid, err := uuid.Parse(caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse case id")
	}
	existingCaseReviewFiles, err := uc.caseReviewFileRepository.ListCaseReviewFiles(ctx, exec, caseIdUuid)
	if err != nil {
		return nil, errors.Wrap(err, "could not list case review files")
	}

	if len(existingCaseReviewFiles) == 0 {
		return nil, nil
	}

	reviewWithFeedbackDto, err := uc.getCaseReviewById(ctx, existingCaseReviewFiles[0].Id)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case review by id")
	}

	return []agent_dto.AiCaseReviewOutputDto{reviewWithFeedbackDto}, nil
}

// Returns a slice of 0 or 1 case review dto, the most recent one.
func (uc *AiAgentUsecase) GetCaseReview(ctx context.Context, caseId string) ([]agent_dto.AiCaseReviewOutputDto, error) {
	_, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return nil, err
	}

	existingReviewDtos, err := uc.getMostRecentCaseReview(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not list case review dtos")
	}

	if len(existingReviewDtos) == 0 {
		return make([]agent_dto.AiCaseReviewOutputDto, 0), nil
	}

	return existingReviewDtos[:1], nil
}

// ListCaseReviews returns all case reviews for a case, ordered by created_at DESC.
// For completed reviews the full review content is included; for others, Review is nil.
func (uc *AiAgentUsecase) ListCaseReviews(ctx context.Context, caseId uuid.UUID) ([]agent_dto.AiCaseReviewListItemDto, error) {
	_, err := uc.getCaseWithPermissions(ctx, caseId.String())
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, errors.Wrap(err, "could not parse case id")
	}

	exec := uc.executorFactory.NewExecutor()
	reviews, err := uc.caseReviewFileRepository.ListAllCaseReviewFiles(ctx, exec, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not list case review files")
	}

	result := make([]agent_dto.AiCaseReviewListItemDto, 0, len(reviews))
	for _, review := range reviews {
		var reaction *string
		if review.Reaction != nil {
			reaction = utils.Ptr(review.Reaction.String())
		}

		item := agent_dto.AiCaseReviewListItemDto{
			Id:        review.Id,
			CaseId:    review.CaseId,
			Status:    review.Status.String(),
			CreatedAt: review.CreatedAt,
			UpdatedAt: review.UpdatedAt,
			Reaction:  reaction,
		}

		if review.Status == models.AiCaseReviewStatusCompleted {
			blob, err := uc.blobRepository.GetBlob(ctx, review.BucketName, review.FileReference)
			if err != nil {
				return nil, errors.Wrap(err, "could not get case review file")
			}
			reviewDto, err := agent_dto.UnmarshalCaseReviewDto(review.DtoVersion, blob.ReadCloser)
			blob.ReadCloser.Close()
			if err != nil {
				return nil, errors.Wrap(err, "could not unmarshal case review file")
			}
			item.Review = reviewDto
		}

		result = append(result, item)
	}

	return result, nil
}

func (uc *AiAgentUsecase) GetCaseReviewById(ctx context.Context, caseId string, reviewId uuid.UUID) (agent_dto.AiCaseReviewOutputDto, error) {
	_, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{}, err
	}

	reviewWithFeedbackDto, err := uc.getCaseReviewById(ctx, reviewId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{},
			errors.Wrap(err, "could not get case review by id")
	}

	return reviewWithFeedbackDto, nil
}

// EnqueueCreateCaseReview enqueues a case review task for a given case.
func (uc *AiAgentUsecase) EnqueueCreateCaseReview(ctx context.Context, caseId string) (bool, error) {
	c, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return false, err
	}

	hasAiCaseReviewEnabled, err := uc.HasAiCaseReviewEnabled(ctx, c.OrganizationId)
	if err != nil {
		return false, errors.Wrap(err, "error checking if AI case review is enabled")
	}
	if !hasAiCaseReviewEnabled {
		return false, nil
	}
	inbox, err := uc.inboxReader.GetInboxById(ctx, uc.executorFactory.NewExecutor(), c.InboxId)
	if err != nil {
		return false, errors.Wrap(err, "error getting inbox")
	}
	if !inbox.CaseReviewManual {
		return false, nil
	}

	caseIdUuid, err := uuid.Parse(caseId)
	if err != nil {
		return false, errors.Wrap(err, "could not parse case id")
	}

	caseReviewId := uuid.Must(uuid.NewV7())
	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return uc.caseReviewTaskEnqueuer.EnqueueCaseReviewTask(ctx, tx, c.OrganizationId, caseIdUuid, caseReviewId)
	})
	return true, err
}

// Return a list of instructions to give to the LLM and the model to use for the prompt
// NOTE: The model is given by `prepareRequest()`, we call it at most twice and the the last call will set the model
func (uc *AiAgentUsecase) getOrganizationInstructionsForPrompt(ctx context.Context,
	customInstructions customOrgInstructions,
) ([]string, string) {
	logger := utils.LoggerFromContext(ctx)
	instructions := []string{}
	modelToUse := ""

	if customInstructions.Language != nil {
		language, err := pure_utils.BCP47ToEnglish(*customInstructions.Language)
		if err != nil {
			logger.DebugContext(ctx, "could not convert language to english, do not format the output with language", "error", err)
		} else {
			model, customLanguagePrompt, err := uc.preparePromptWithModel(
				INSTRUCTION_LANGUAGE_PATH,
				map[string]any{
					"language": language,
				},
			)
			if err != nil {
				logger.DebugContext(ctx, "could not read custom language prompt", "error", err)
			} else {
				instructions = append(instructions, customLanguagePrompt)
			}

			modelToUse = model
		}
	}
	if customInstructions.Structure != nil {
		model, customStructurePrompt, err := uc.preparePromptWithModel(
			INSTRUCTION_STRUCTURE_PATH,
			map[string]any{
				"structure": *customInstructions.Structure,
			},
		)
		if err != nil {
			logger.DebugContext(ctx, "could not read custom structure prompt", "error", err)
		} else {
			instructions = append(instructions, customStructurePrompt)
		}

		modelToUse = model
	}

	return instructions, modelToUse
}

// Contains all results from the case review process
// Update this struct during the process and expose this struct to the caller to save the results in case we need to resume it
// Does not include the custom format output because it's the last step and we don't need to save it.
type CaseReviewContext struct {
	DataModelSummary       *string                  `json:"data_model_summary"`
	FieldsToReadPerTable   map[string][]string      `json:"fields_to_read_per_table"`
	RulesDefinitionsReview *string                  `json:"rules_definitions_review"`
	RuleThresholds         *string                  `json:"rule_thresholds"`
	PivotEnrichments       []models.AiEnrichmentKYC `json:"pivot_enrichments"`
	CaseReview             *caseReviewOutput        `json:"case_review"`
	SanityCheck            *sanityCheckOutput       `json:"sanity_check"`
}

// CreateCaseReviewSync performs a comprehensive AI-powered review of a case by analyzing
// case data, related information, and generating structured insights through multiple
// AI model interactions. The process involves several key steps:
// 1. Initialize AI client and gather case data with security validation
// 2. Generate data model summary to understand the case structure
// 3. Determine optimal fields to read for large tables to manage data volume
// 4. Analyze rule definitions and thresholds for context
// 5. Enrich the pivot data with internet research
// 6. Generate the main case review with all available information
// 7. Perform sanity check on the generated review for quality assurance
// 8. Format the output with the custom instructions if given (language or structure for example)
// caseReviewContext can be provided to resume the process from a previous iteration, it will be updated in place
// Depends on the context, the process can avoid calling some LLM calls
func (uc *AiAgentUsecase) CreateCaseReviewSync(
	ctx context.Context,
	caseId string,
	caseReviewContext *CaseReviewContext,
) (agent_dto.AiCaseReviewDto, error) {
	logger := utils.LoggerFromContext(ctx)

	if caseReviewContext == nil {
		caseReviewContext = &CaseReviewContext{}
	}

	client, err := uc.GetClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not create ai client")
	}

	caseData, relatedDataPerClient, err := uc.getCaseDataWithPermissions(ctx, caseId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get case data")
	}

	// Check if the organization has enough funds to cover the cost of the case review
	enoughFunds, subscriptionId, err := uc.billingUsecase.CheckIfEnoughFundsInWallet(
		ctx,
		caseData.organizationId,
		billing.AI_CASE_REVIEW)
	if err != nil {
		return nil, errors.Wrap(err, "could not check if enough funds in wallet")
	}
	if !enoughFunds {
		return nil, billing.ErrInsufficientFunds
	}

	// Get AI setting
	aiSetting, err := uc.getAiSetting(ctx, caseData.organizationId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get ai setting")
	}

	// Prepare the custom org instructions
	customOrgInstructions := getOrganizationCustomInstructions(aiSetting)
	organizationDescription := "No description provided"
	if customOrgInstructions.OrgDescription != nil {
		organizationDescription = *customOrgInstructions.OrgDescription
	}

	// Define the system instruction for prompt
	systemInstruction, err := readPrompt(SYSTEM_PROMPT_PATH)
	if err != nil {
		logger.DebugContext(ctx, "could not read system instruction", "error", err)
		systemInstruction = "You are a compliance officer or fraud analyst. You are given a case and you need to review it step by step. Reply factually to instructions in markdown format."
	}

	if caseReviewContext.DataModelSummary == nil {
		// Data model summary, create thread because the response will be used in next steps
		modelDataModelSummary, promptDataModelSummary, err := uc.preparePromptWithModel(
			PROMPT_DATA_MODEL_SUMMARY_PATH, map[string]any{
				"data_model": caseData.dataModelDto,
			})
		if err != nil {
			return nil, errors.Wrap(err, "could not prepare data model summary request")
		}

		// Create the request with Thread for the next steps which needs the response
		requestDataModelSummary, err := llmberjack.NewUntypedRequest().
			WithModel(modelDataModelSummary).
			WithThinking(false).
			WithInstruction(systemInstruction).
			WithText(llmberjack.RoleUser, promptDataModelSummary).
			Do(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "could not generate data model summary")
		}

		dataModelSummary, err := requestDataModelSummary.Get(0)
		if err != nil {
			return nil, errors.Wrap(err, "could not get data model summary")
		}
		caseReviewContext.DataModelSummary = &dataModelSummary
	}

	logger.DebugContext(ctx, "================================ Data model summary ================================")
	logger.DebugContext(ctx, "Data model summary", "response", *caseReviewContext.DataModelSummary)

	// Data model object field read options
	// Here, we implicitly distinguish between "transaction" tables (based on the presence of "many" rows for a given customer)
	// and other tables (where we can afford to read all fields)
	allPresentTables := make(map[string]bool)
	for _, clientData := range relatedDataPerClient.ingestedData {
		for tableName := range clientData {
			allPresentTables[tableName] = true
		}
	}

	tablesWithLargRowNbs := make(map[string][]string)
	allTables := caseData.dataModel.Tables
	for tableName := range allPresentTables {
		if someClientHasManyRowsForTable(relatedDataPerClient.ingestedData, tableName) {
			tablesWithLargRowNbs[tableName] = allTables[tableName].FieldNames()
		}
	}
	tableNamesWithLargRowNbs := pure_utils.Keys(tablesWithLargRowNbs)

	if len(tablesWithLargRowNbs) > 0 {
		for pivotObjectKey, objectTables := range relatedDataPerClient.ingestedData {
			if len(objectTables) > 0 {
				// generate the map of fields to read for every table, but only once.
				if caseReviewContext.FieldsToReadPerTable == nil {
					props := jsonschema.NewProperties()

					for tableName, fields := range tablesWithLargRowNbs {
						props.Set(tableName, &jsonschema.Schema{
							Type: "array",
							Items: &jsonschema.Schema{
								Type: "string",
								Enum: pure_utils.ToAnySlice(fields),
							},
						})
					}

					schema := jsonschema.Schema{
						Type:       "object",
						Properties: props,
					}

					modelDataModelObjectFieldReadOptions, promptDataModelObjectFieldReadOptions, err := uc.preparePromptWithModel(
						PROMPT_DATA_MODEL_OBJECT_FIELD_READ_OPTIONS_PATH,
						map[string]any{
							"data_model_table_names": tableNamesWithLargRowNbs,
						},
					)
					if err != nil {
						return nil, errors.Wrap(err, "could not prepare data model object field read options request")
					}

					requestDataModelObjectFieldReadOptions, err := llmberjack.NewRequest[map[string][]string]().
						OverrideResponseSchema(schema).
						WithModel(modelDataModelObjectFieldReadOptions).
						WithThinking(false).
						WithInstruction(systemInstruction).
						WithText(llmberjack.RoleAi, *caseReviewContext.DataModelSummary).
						WithText(llmberjack.RoleUser, promptDataModelObjectFieldReadOptions).
						Do(ctx, client)
					if err != nil {
						return nil, errors.Wrap(err, "could not generate data model object field read options")
					}

					dataModelObjectFieldReadOptions, err :=
						requestDataModelObjectFieldReadOptions.Get(0)
					if err != nil {
						return nil, errors.Wrap(err, "could not get data model object field read options")
					}

					logger.DebugContext(ctx, "================================ Data model object field read options ================================")
					logger.DebugContext(ctx, "Data model object field read options",
						"response", dataModelObjectFieldReadOptions)

					caseReviewContext.FieldsToReadPerTable = dataModelObjectFieldReadOptions
				}

				for tableName, fieldsToRead := range caseReviewContext.FieldsToReadPerTable {
					// Reuse original read options, just adapt the number of rows to read and the fields to consider
					fieldFilteredObjects, _, _, err := uc.ingestedDataReader.ReadIngestedClientObjects(
						ctx,
						caseData.organizationId,
						tableName,
						models.ClientDataListRequestBody{
							ExplorationOptions: objectTables[tableName].ReadOptions,
							Limit:              500,
						},
						fieldsToRead...,
					)
					if err != nil {
						return nil, errors.Wrapf(err,
							"could not read ingested client objects for %s", tableName)
					}
					// then, update the ingested data for this pivot object/table combination with the new filtered data
					relatedDataPerClient.ingestedData[pivotObjectKey][tableName] = agent_dto.IngestedDataResult{
						Data:        fieldFilteredObjects,
						ReadOptions: objectTables[tableName].ReadOptions,
					}
				}

			}
		}
	}

	if caseReviewContext.RulesDefinitionsReview == nil {

		// Rules definitions review
		modelRulesDefinitions, promptRulesDefinitions, err := uc.preparePromptWithModel(
			PROMPT_RULE_DEFINITIONS_PATH,
			map[string]any{
				"decisions":            caseData.decisions,
				"activity_description": organizationDescription,
			},
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not prepare rules definitions review request")
		}
		requestRulesDefinitionsReview, err := llmberjack.NewUntypedRequest().
			WithModel(modelRulesDefinitions).
			WithThinking(false).
			WithInstruction(systemInstruction).
			WithText(llmberjack.RoleUser, promptRulesDefinitions).
			Do(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "could not generate rules definitions review")
		}
		rulesDefinitionsReview, err := requestRulesDefinitionsReview.Get(0)
		if err != nil {
			return nil, errors.Wrap(err, "could not get rules definitions review")
		}
		caseReviewContext.RulesDefinitionsReview = &rulesDefinitionsReview
	}
	logger.DebugContext(ctx, "================================ Rules definitions review ================================")
	logger.DebugContext(ctx, "Rules definitions review", "response",
		*caseReviewContext.RulesDefinitionsReview)

	// Rule thresholds
	if caseReviewContext.RuleThresholds == nil {
		modelRuleThresholds, promptRuleThresholds, err := uc.preparePromptWithModel(
			PROMPT_RULE_THRESHOLD_VALUES_PATH,
			map[string]any{
				"decisions": caseData.decisions,
			},
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not prepare rule thresholds request")
		}
		requestRuleThresholds, err := llmberjack.NewUntypedRequest().
			WithModel(modelRuleThresholds).
			WithThinking(false).
			WithInstruction(systemInstruction).
			WithText(llmberjack.RoleUser, promptRuleThresholds).
			Do(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "could not generate rule thresholds")
		}
		ruleThresholds, err := requestRuleThresholds.Get(0)
		if err != nil {
			return nil, errors.Wrap(err, "could not get rule thresholds")
		}
		caseReviewContext.RuleThresholds = &ruleThresholds
	}

	logger.DebugContext(ctx, "================================ Rule thresholds ================================")
	logger.DebugContext(ctx, "Rule thresholds", "response", *caseReviewContext.RuleThresholds)

	if len(caseReviewContext.PivotEnrichments) == 0 {
		enrichmentResults, err := uc.EnrichCasePivotObjects(ctx, caseData.organizationId, caseId)
		if err != nil {
			if !errors.Is(err, ErrKYCEnrichmentNotEnabled) {
				return nil, errors.Wrap(err, "could not enrich case pivot objects")
			}
			logger.DebugContext(ctx, "KYC enrichment is not enabled, skip enrichment")
		}

		caseReviewContext.PivotEnrichments = enrichmentResults
	}

	logger.DebugContext(ctx, "================================ Pivot enrichments ================================")
	logger.DebugContext(ctx, "Pivot enrichments", "response", caseReviewContext.PivotEnrichments)

	// Finally, we can generate the case review
	if caseReviewContext.CaseReview == nil {
		var astOptions []ast.FuncAttributes
		for _, funcAttributes := range ast.FuncAttributesMap {
			astOptions = append(astOptions, funcAttributes)
		}
		astOptionsBytes, err := json.Marshal(astOptions)
		if err != nil {
			return nil, errors.Wrap(err, "could not marshal ast options")
		}
		modelCaseReview, promptCaseReview, err := uc.preparePromptWithModel(
			PROMPT_CASE_REVIEW_PATH,
			map[string]any{
				// Global data
				"org_activity":     organizationDescription,
				"rule_ast_options": string(astOptionsBytes),

				// Case data
				"case_detail":           caseData.case_,
				"case_events":           caseData.events,
				"decisions":             caseData.decisions,
				"has_more_alerts":       caseData.hasMoreDecisions,
				"pivot_objects":         caseData.pivotData,
				"previous_cases":        relatedDataPerClient.relatedCases,
				"customer_related_data": relatedDataPerClient.ingestedData,

				// Data from previous steps
				"data_model_summary": caseReviewContext.DataModelSummary,
				"rules_summary":      caseReviewContext.RulesDefinitionsReview,
				"rule_thresholds":    caseReviewContext.RuleThresholds,
				"pivot_enrichments":  caseReviewContext.PivotEnrichments,
			},
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not prepare case review request")
		}

		schema := getProofSchema(caseData.dataModel)
		requestCaseReview, err := llmberjack.NewRequest[caseReviewOutput]().
			OverrideResponseSchema(schema).
			WithModel(modelCaseReview).
			WithInstruction(systemInstruction).
			WithText(llmberjack.RoleUser, promptCaseReview).
			Do(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "could not generate case review")
		}
		caseReview, err := requestCaseReview.Get(0)
		if err != nil {
			return nil, errors.Wrap(err, "could not get case review")
		}

		// any incorrect proofs are logged but otherwise ignored
		validProofs := make([]proof, 0, len(caseReview.Proofs))
		for _, proof := range caseReview.Proofs {
			badObject, err := uc.shouldRemoveProof(ctx, caseData, proof)
			if err != nil {
				return nil, err
			}
			if !badObject {
				validProofs = append(validProofs, proof)
			}
		}
		caseReview.Proofs = validProofs

		caseReviewContext.CaseReview = &caseReview
	}
	logger.DebugContext(ctx, "================================ Full case review ================================")
	logger.DebugContext(ctx, "Full case review", "response", *caseReviewContext.CaseReview)

	if caseReviewContext.SanityCheck == nil {
		// Finally, sanity check the resulting case review using a judgement prompt
		modelSanityCheck, promptSanityCheck, err := uc.preparePromptWithModel(
			PROMPT_SANITY_CHECK_PATH,
			map[string]any{
				// Global data
				"org_activity": organizationDescription,

				// Case data
				"case_detail":           caseData.case_,
				"case_events":           caseData.events,
				"decisions":             caseData.decisions,
				"pivot_objects":         caseData.pivotData,
				"has_more_alerts":       caseData.hasMoreDecisions,
				"previous_cases":        relatedDataPerClient.relatedCases,
				"customer_related_data": relatedDataPerClient.ingestedData,

				// Data from previous steps
				"data_model_summary": caseReviewContext.DataModelSummary,
				"rules_summary":      caseReviewContext.RulesDefinitionsReview,
				"rule_thresholds":    caseReviewContext.RuleThresholds,
				"pivot_enrichments":  caseReviewContext.PivotEnrichments,
				"case_review":        caseReviewContext.CaseReview,
			},
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not prepare sanity check request")
		}
		requestSanityCheck, err := llmberjack.NewRequest[sanityCheckOutput]().
			WithModel(modelSanityCheck).
			WithInstruction(systemInstruction).
			WithText(llmberjack.RoleUser, promptSanityCheck).
			Do(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "could not generate sanity check")
		}
		sanityCheck, err := requestSanityCheck.Get(0)
		if err != nil {
			return nil, errors.Wrap(err, "could not get sanity check")
		}
		caseReviewContext.SanityCheck = &sanityCheck
	}

	logger.DebugContext(ctx, "================================ Sanity check ================================")
	logger.DebugContext(ctx, "Sanity check", "response", *caseReviewContext.SanityCheck)

	// Do custom language and structure instructions
	finalOutput := caseReviewContext.CaseReview.CaseReview

	instructions, modelForInstruction := uc.getOrganizationInstructionsForPrompt(ctx, customOrgInstructions)

	// If there are instructions, we need to format the output
	if len(instructions) == 0 {
		logger.DebugContext(ctx, "No custom instructions for organization, skip this part")
	} else {
		customReportInstruction, err := readPrompt(INSTRUCTION_CUSTOM_REPORT_PATH)
		if err != nil {
			logger.DebugContext(ctx, "could not read custom report instruction", "error", err)
			customReportInstruction = "Transform the case review according to the instructions. Return only the transformed content without explanations or preambles."
		}

		customFormatRequest := llmberjack.NewRequest[string]().
			WithModel(modelForInstruction).
			WithInstruction(systemInstruction).
			WithThinking(false).
			WithInstruction(customReportInstruction)
		// Add all custom instructions for organization
		for _, instruction := range instructions {
			logger.DebugContext(ctx, "Adding custom instruction", "instruction", instruction)
			customFormatRequest = customFormatRequest.WithInstruction(instruction)
		}
		requestCustomFormat, err := customFormatRequest.
			WithText(llmberjack.RoleUser, finalOutput).
			Do(ctx, client)
		if err != nil {
			return nil, errors.Wrap(err, "could not execute custom format")
		}
		finalOutput, err = requestCustomFormat.Get(0)
		if err != nil {
			return nil, errors.Wrap(err, "could not get custom format from response")
		}

		logger.DebugContext(ctx, "================================ Custom format ================================")
		logger.DebugContext(ctx, "Custom format", "response", finalOutput)
	}

	// Format the proof evidence
	proofs := make([]agent_dto.CaseReviewProof, len(caseReviewContext.CaseReview.Proofs))
	for i, proof := range caseReviewContext.CaseReview.Proofs {
		proofs[i] = agent_dto.CaseReviewProof{
			Id:     proof.Id,
			Type:   proof.Type,
			Origin: proof.Origin,
			Reason: proof.Reason,
		}
	}
	var pivotEnrichments *agent_dto.KYCEnrichmentResultsDto
	if len(caseReviewContext.PivotEnrichments) > 0 {
		pivotEnrichments = utils.Ptr(agent_dto.AdaptKYCEnrichmentResultsDto(caseReviewContext.PivotEnrichments))
	}

	// Send billing event
	err = uc.billingUsecase.EnqueueBillingEventTask(ctx, models.BillingEvent{
		TransactionId:          uuid.Must(uuid.NewV7()).String(),
		ExternalSubscriptionId: subscriptionId,
		Code:                   billing.AI_CASE_REVIEW.String(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not send billing event")
	}

	// Can access to Ok and Justification, the nil check is done in the sanity check step
	if caseReviewContext.SanityCheck.Ok {
		// should not happen because there is an enum on the jsonschema, but check anyway
		if caseReviewContext.SanityCheck.ReviewLevel != nil &&
			!slices.Contains(ReviewLevelEnum, *caseReviewContext.SanityCheck.ReviewLevel) {
			logger.ErrorContext(ctx,
				"invalid review level received: "+*caseReviewContext.SanityCheck.ReviewLevel)
			caseReviewContext.SanityCheck.ReviewLevel = nil
		} else if caseReviewContext.SanityCheck.ReviewLevel == nil {
			logger.ErrorContext(ctx, "no review level received")
		}
		return agent_dto.CaseReviewV1{
			Ok:               caseReviewContext.SanityCheck.Ok,
			Output:           finalOutput,
			Proofs:           proofs,
			PivotEnrichments: pivotEnrichments,
			ReviewLevel:      caseReviewContext.SanityCheck.ReviewLevel,
		}, nil
	}
	return agent_dto.CaseReviewV1{
		Ok:               false,
		Output:           finalOutput,
		SanityCheck:      caseReviewContext.SanityCheck.Justification,
		Proofs:           proofs,
		PivotEnrichments: pivotEnrichments,
	}, nil
}

type casePivotDataByPivot struct {
	ingestedData agent_dto.CaseIngestedDataByPivot
	relatedCases map[string][]agent_dto.CaseWithDecisions
}

func (uc *AiAgentUsecase) getCaseDataWithPermissions(ctx context.Context, caseId string) (caseData, casePivotDataByPivot, error) {
	exec := uc.executorFactory.NewExecutor()
	c, err := uc.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return caseData{}, casePivotDataByPivot{}, err
	}

	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, c.OrganizationId, false)
	if err != nil {
		return caseData{}, casePivotDataByPivot{},
			errors.Wrap(err, "failed to list available inboxes in usecase")
	}
	availableInboxIds := make([]uuid.UUID, len(inboxes))
	for i, inbox := range inboxes {
		availableInboxIds[i] = inbox.Id
	}

	if err := uc.enforceSecurityCase.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return caseData{}, casePivotDataByPivot{}, err
	}

	tags, err := uc.repository.ListOrganizationTags(ctx, exec, c.OrganizationId, models.TagTargetCase, false)
	if err != nil {
		return caseData{}, casePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve tags for case")
	}
	caseEvents, err := uc.repository.ListCaseEvents(ctx, exec, caseId)
	if err != nil {
		return caseData{}, casePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve case events")
	}

	users, err := uc.repository.ListUsers(ctx, exec, &c.OrganizationId)
	if err != nil {
		return caseData{}, casePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve users for case events")
	}
	caseEventsDto := make([]agent_dto.CaseEvent, len(caseEvents))
	for i := range caseEvents {
		caseEventsDto[i] = agent_dto.AdaptCaseEventDto(caseEvents[i], users)
	}

	decisions, _, err := uc.repository.DecisionsByCaseIdFromCursor(ctx, exec, models.CaseDecisionsRequest{
		OrgId:  c.OrganizationId,
		CaseId: caseId,
		Limit:  models.CaseDecisionsPerPage,
	})
	if err != nil {
		return caseData{}, casePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve case decisions")
	}

	decicionsWithRulesExec, err := uc.repository.DecisionsWithRuleExecutionsByIds(ctx, exec,
		pure_utils.Map(decisions, func(d models.DecisionWithRulesAndScreeningsBaseInfo) string { return d.DecisionId.String() }))
	if err != nil {
		return caseData{}, casePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve case decisions with rule executions")
	}

	hasMoreDecisions := false

	decisionDtos := make([]agent_dto.Decision, 0, MAX_DECISIONS_REVIEW_PER_CASE)
	for i, decision := range decicionsWithRulesExec {
		// We take only the MAX_DECISIONS_REVIEW_PER_CASE first decisions, in order not to overload the context for large cases.
		// The template handles a "has_more_alerts" boolean flag to indicate this in the prompt when it happens.
		if i >= MAX_DECISIONS_REVIEW_PER_CASE {
			hasMoreDecisions = true
			break
		}
		iteration, err := uc.repository.GetScenarioIteration(ctx, exec,
			decision.Decision.ScenarioIterationId.String(), true)
		if err != nil {
			return caseData{}, casePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve scenario for decision %s", decision.DecisionId)
		}
		rules, err := uc.repository.ListRulesByIterationId(ctx, exec,
			decision.Decision.ScenarioIterationId.String())
		if err != nil {
			return caseData{}, casePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve rules for decision %s", decision.DecisionId)
		}
		screenings, err := uc.repository.ListScreeningsForDecision(ctx, exec, decision.DecisionId.String(), true)
		if err != nil {
			return caseData{}, casePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve screenings for decision %s", decision.DecisionId)
		}
		decisionDtos = append(decisionDtos, agent_dto.AdaptDecision(
			decision.Decision,
			decision.RuleExecutions,
			screenings,
			iteration,
			rules,
		))
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, c.OrganizationId, models.DataModelReadOptions{
		IncludeEnums: true, IncludeNavigationOptions: true,
	}, true)
	if err != nil {
		return caseData{}, casePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve data model")
	}

	pivotValues, err := uc.repository.DecisionPivotValuesByCase(ctx, exec, caseId)
	if err != nil {
		return caseData{}, casePivotDataByPivot{}, err
	}
	pivotObjects, err := uc.ingestedDataReader.ReadPivotObjectsFromValues(ctx, c.OrganizationId, pivotValues)
	if err != nil {
		return caseData{}, casePivotDataByPivot{},
			errors.Wrap(err, "could not read pivot objects from values")
	}
	pivotObjectDtos, err := pure_utils.MapErr(pivotObjects, agent_dto.AdaptPivotObjectDto)
	if err != nil {
		return caseData{}, casePivotDataByPivot{},
			errors.Wrap(err, "could not adapt pivot objects to DTOs")
	}

	relatedDataPerClient := casePivotDataByPivot{
		ingestedData: make(agent_dto.CaseIngestedDataByPivot, len(pivotObjects)),
		relatedCases: make(map[string][]agent_dto.CaseWithDecisions, len(pivotObjects)),
	}

	for _, pivotObject := range pivotObjects {
		pivotObjectKey := agent_dto.PivotObjectKeyForMap(pivotObject)

		// This map is a map of [tableName]IngestedDataResult for this pivot object
		relatedDataPerClient.ingestedData[pivotObjectKey] =
			make(agent_dto.CasePivotIngestedData, len(dataModel.Tables))

		relatedDataPerClient.relatedCases[pivotObjectKey] =
			make([]agent_dto.CaseWithDecisions, 0, 10)

		previousCases, err := uc.repository.GetCasesWithPivotValue(ctx, exec,
			c.OrganizationId, pivotObject.PivotValue)
		if err != nil {
			return caseData{}, casePivotDataByPivot{}, err
		}

		// first, generate the list of related cases for this pivot object
		for i, previousCase := range previousCases {
			if previousCase.Id == c.Id {
				// skip the current case, we don't want to include it in the related cases
				continue
			}
			// We take only the MAX_PREVIOUS_CASES_REVIEW_PER_PIVOT_OBJECT first previous cases, in order not to overload the context for large cases.
			if i >= MAX_PREVIOUS_CASES_REVIEW_PER_PIVOT_OBJECT {
				break
			}

			// We take only the MAX_DECISIONS_FROM_PREVIOUS_CASES_REVIEW first decisions from each previous case, in order not to overload the context for large cases.
			decisions, _, err := uc.repository.DecisionsByCaseIdFromCursor(ctx, exec, models.CaseDecisionsRequest{
				OrgId:  c.OrganizationId,
				CaseId: previousCase.Id,
				Limit:  MAX_DECISIONS_FROM_PREVIOUS_CASES_REVIEW,
			})
			if err != nil {
				return caseData{}, casePivotDataByPivot{}, errors.Wrapf(err,
					"could not retrieve decisions for previous case %s", previousCase.Id)
			}

			events, err := uc.repository.ListCaseEvents(ctx, exec, previousCase.Id)
			if err != nil {
				return caseData{}, casePivotDataByPivot{}, err
			}
			previousCase.Events = events

			// Use case DTO without rule execution details and without screening details, in order not to
			// overload the context with too much data from old cases
			rc, err := agent_dto.AdaptCaseWithDecisionsDtoWithoutRuleExecDetails(
				previousCase,
				decisions,
				tags, inboxes, users,
				func(scenarioIterationId string) (models.ScenarioIteration, error) {
					return uc.repository.GetScenarioIteration(ctx, exec, scenarioIterationId, true)
				},
			)
			if err != nil {
				return caseData{}, casePivotDataByPivot{}, errors.Wrapf(err,
					"could not adapt case with decisions for previous case %s", previousCase.Id)
			}
			relatedDataPerClient.relatedCases[pivotObjectKey] =
				append(relatedDataPerClient.relatedCases[pivotObjectKey], rc)
		}

		// then, retrieve the ingested data for this pivot object (of any navigation options exist)
		// This will only retrieve data from tables reachable through "one to many" relationships
		navigationOptions := dataModel.Tables[pivotObject.PivotObjectName].NavigationOptions
		for _, navOption := range navigationOptions {
			if _, found := relatedDataPerClient.ingestedData[pivotObjectKey][navOption.TargetTableName]; found {
				// If we already have data for this target table, skip it. Multiple navigation options
				// to the same table should be the exception.
				continue
			}
			sourceFieldValue, ok := pivotObject.PivotObjectData.Data[navOption.SourceFieldName]
			if !ok {
				continue
			}
			sourceFieldValueStr, ok := sourceFieldValue.(string)
			if !ok {
				continue
			}
			if navOption.Status == models.IndexStatusValid {
				readOptions := models.ExplorationOptions{
					SourceTableName:   pivotObject.PivotObjectName,
					FilterFieldName:   navOption.FilterFieldName,
					FilterFieldValue:  models.NewStringOrNumberFromString(sourceFieldValueStr),
					OrderingFieldName: navOption.OrderingFieldName,
				}
				objects, _, _, err := uc.ingestedDataReader.ReadIngestedClientObjects(ctx,
					c.OrganizationId, navOption.TargetTableName, models.ClientDataListRequestBody{
						ExplorationOptions: readOptions,
						Limit:              1000,
					})
				if err != nil {
					return caseData{}, casePivotDataByPivot{}, errors.Wrapf(err,
						"could not read ingested client objects for %s with value %s",
						pivotObject.PivotObjectName, sourceFieldValueStr)
				}
				relatedDataPerClient.ingestedData[pivotObjectKey][navOption.TargetTableName] = agent_dto.IngestedDataResult{
					Data:        objects,
					ReadOptions: readOptions,
				}
			}
		}
	}

	return caseData{
		case_:            agent_dto.AdaptCaseDto(c, tags, inboxes, users),
		events:           caseEventsDto,
		decisions:        decisionDtos,
		hasMoreDecisions: hasMoreDecisions,
		dataModelDto:     agent_dto.AdaptDataModelDto(dataModel),
		dataModel:        dataModel,
		pivotData:        pivotObjectDtos,
		organizationId:   c.OrganizationId,
	}, relatedDataPerClient, nil
}

type caseData struct {
	case_            agent_dto.Case
	hasMoreDecisions bool
	events           []agent_dto.CaseEvent
	decisions        []agent_dto.Decision
	dataModelDto     agent_dto.DataModel
	dataModel        models.DataModel
	pivotData        []agent_dto.PivotObject
	organizationId   uuid.UUID
}

func someClientHasManyRowsForTable(relatedDataPerClient agent_dto.CaseIngestedDataByPivot, tableName string) bool {
	for _, clientData := range relatedDataPerClient {
		if clientData[tableName].Data != nil {
			return len(clientData[tableName].Data) > HIGH_NB_ROWS_THRESHOLD
		}
	}
	return false
}

func (uc *AiAgentUsecase) UpdateAiCaseReviewFeedback(
	ctx context.Context,
	caseId string,
	reviewId uuid.UUID,
	feedback models.AiCaseReviewFeedback,
) (agent_dto.AiCaseReviewOutputDto, error) {
	exec := uc.executorFactory.NewExecutor()

	_, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{}, err
	}

	if err := uc.repository.UpdateAiCaseReviewFeedback(ctx, exec, reviewId, feedback); err != nil {
		return agent_dto.AiCaseReviewOutputDto{}, err
	}

	caseReview, err := uc.getCaseReviewById(ctx, reviewId)
	if err != nil {
		return agent_dto.AiCaseReviewOutputDto{}, err
	}

	return caseReview, nil
}

func (uc *AiAgentUsecase) HasAiCaseReviewEnabled(ctx context.Context, orgId uuid.UUID) (bool, error) {
	featureAccess, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return false, err
	}
	return featureAccess.CaseAiAssist.IsAllowed(), nil
}

var templateFuncMap = template.FuncMap{
	"isTrue": func(v any) bool {
		// 1. Try to convert the `any` value to a `*bool`.
		b, ok := v.(*bool)

		// 2. If it's not a *bool OR if the pointer is nil, it's not "true".
		if !ok || b == nil {
			return false
		}

		// 3. If we're here, we have a valid pointer. Dereference it and return its value.
		return *b
	},
}

func isPivotObject(proof agent_dto.CaseReviewProof, pivotObjects []agent_dto.PivotObject) bool {
	for _, pivotObject := range pivotObjects {
		if pivotObject.PivotObjectName == proof.Type && pivotObject.PivotValue == proof.Id {
			return true
		}
	}
	return false
}

func isDecisionTriggerObject(proof agent_dto.CaseReviewProof, decisions []agent_dto.Decision) bool {
	for _, decision := range decisions {
		if decision.TriggerObjectType == proof.Type &&
			decision.TriggerObject["object_id"] == proof.Id {
			return true
		}
	}
	return false
}

// shouldRemoveProof centralizes validation of a proof element emitted by the LLM.
// It returns true if the proof should be filtered out, and an error if a lookup failed.
func (uc *AiAgentUsecase) shouldRemoveProof(ctx context.Context, caseData caseData, p proof) (bool, error) {
	logger := utils.LoggerFromContext(ctx)

	if p.Origin != agent_dto.OriginNameDataModel {
		logger.ErrorContext(ctx, "invalid proof origin", "origin", p.Origin)
		return true, nil
	}

	objs, err := uc.ingestedDataReader.GetIngestedObject(
		ctx, caseData.organizationId,
		&caseData.dataModel, p.Type,
		p.Id, "object_id",
	)
	if err != nil {
		return false, errors.Wrapf(err, "could not get ingested object of type %s for proof %s", p.Type, p.Id)
	}
	if len(objs) == 0 {
		// Reuse existing helpers for nuanced logging
		dtoProof := agent_dto.CaseReviewProof{Id: p.Id, Type: p.Type, Origin: p.Origin}
		if isPivotObject(dtoProof, caseData.pivotData) {
			// The agent "should not" use this as proof, it's already the basic data from the case. Silently ignore it if it happens.
			logger.DebugContext(ctx, fmt.Sprintf("pivot object %s \"%s\" used as proof but has not been ingested", p.Type, p.Id))
		} else if isDecisionTriggerObject(dtoProof, caseData.decisions) {
			// TODO: ideally, allow this and make the frontend handle this case, by having a different type of proof data
			// "decision_trigger". In practice, not the highest priority so may be done later.
			logger.DebugContext(ctx, fmt.Sprintf("decision trigger object %s \"%s\" used as proof but has not been ingested", p.Type, p.Id))
		} else {
			logger.ErrorContext(ctx, "no ingested object found for proof", "type", p.Type, "id", p.Id)
		}
		return true, nil
	}

	return false, nil
}

func getProofSchema(dataModel models.DataModel) jsonschema.Schema {
	// The schema is constructed dynamically so that we can pass the enum of table names explicitly.
	properties := jsonschema.NewProperties()
	properties.Set("case_review", &jsonschema.Schema{
		Type: "string",
	})
	proofsSchemaDataModel := jsonschema.NewProperties()
	proofsSchemaDataModel.Set("id", &jsonschema.Schema{
		Type:        "string",
		Description: "The ID of the object used as proof. For ingested data, this is their object_id. The value of the id should be consistent with the type of the object, in the sense that an object of type \"{type}\" with id \"{object_id}\" is present in the case.",
	})
	proofsSchemaDataModel.Set("type", &jsonschema.Schema{
		Type:        "string",
		Description: "The type of object that serves as proof.",
		Enum:        pure_utils.ToAnySlice(pure_utils.Keys(dataModel.Tables)),
	})
	proofsSchemaDataModel.Set("origin", &jsonschema.Schema{
		Type:        "string",
		Description: "The origin of the object used as proof. In this case, can only be \"data_model\".",
		Enum:        []any{"data_model"},
	})
	proofsSchemaDataModel.Set("reason", &jsonschema.Schema{
		Type:        "string",
		Description: "The reason why this object was useful for your review.",
	})

	// TODO: later, perhaps add the possibility for "internal" objects, meaning screenings or screening matches, or for external links.
	// For now, the frontend will only handle "data_model" objects.
	properties.Set("proofs", &jsonschema.Schema{
		Type: "array",
		Items: &jsonschema.Schema{
			Type:       "object",
			Properties: proofsSchemaDataModel,
			Required:   []string{"id", "type", "origin", "reason"},
		},
	})
	return jsonschema.Schema{
		Title:       "Case Review",
		Description: "A review of a case",
		Type:        "object",
		Properties:  properties,
	}
}
