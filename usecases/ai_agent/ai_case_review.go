package ai_agent

import (
	"context"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
)

const HIGH_NB_ROWS_THRESHOLD = 100

var ErrAiCaseReviewNotEnabled = errors.New("AI case review is not enabled")

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
	Ok            bool   `json:"ok" jsonschema_description:"Whether the case review is ok or not" jsonschema_required:"true"`
	Justification string `json:"justification" jsonschema_description:"Detailed justification for the sanity check, only in the case of a negative answer" jsonschema_required:"false"`
}

type caseReviewOutput struct {
	CaseReview string `json:"case_review" jsonschema_description:"The case review report in markdown format"`
	Proofs     []struct {
		Id     string               `json:"id" jsonschema_description:"The ID of the object used as proof. For the organization data model, this is referred to as object_id."`
		Type   string               `json:"type" jsonschema_description:"The type of the object used as proof, could be decision or case. For the organization data model, this is referred to as trigger_object_type."`
		Origin agent_dto.OriginName `json:"origin" jsonschema_description:"The origin of the object used as proof, could be data_model or internal"`
		Reason string               `json:"reason" jsonschema_description:"The reason why this object was useful for your review"`
	} `json:"proofs" jsonschema_description:"The proofs used to generate the case review"`
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
// It checks if the organization has AI case review enabled and returns an ErrAiCaseReviewNotEnabled error if not.
func (uc *AiAgentUsecase) EnqueueCreateCaseReview(ctx context.Context, caseId string) error {
	c, err := uc.getCaseWithPermissions(ctx, caseId)
	if err != nil {
		return err
	}

	hasAiCaseReviewEnabled, err := uc.HasAiCaseReviewEnabled(ctx, c.OrganizationId)
	if err != nil {
		return errors.Wrap(err, "error checking if AI case review is enabled")
	}
	if !hasAiCaseReviewEnabled {
		return ErrAiCaseReviewNotEnabled
	}

	caseIdUuid, err := uuid.Parse(caseId)
	if err != nil {
		return errors.Wrap(err, "could not parse case id")
	}

	aiCaseReview := models.NewAiCaseReview(caseIdUuid, uc.caseManagerBucketUrl)
	err = uc.caseReviewFileRepository.CreateCaseReviewFile(ctx,
		uc.executorFactory.NewExecutor(),
		aiCaseReview,
	)
	if err != nil {
		return errors.Wrap(err, "Error while creating case review file")
	}

	return uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return uc.caseReviewTaskEnqueuer.EnqueueCaseReviewTask(ctx, tx, c.OrganizationId, caseIdUuid, aiCaseReview.Id)
	})
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
// Didn't include the sanity check output because it's the last step and we don't need to save it
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

	// Get AI setting
	aiSetting, err := uc.getAiSetting(ctx, caseData.organizationId)
	if err != nil {
		return nil, errors.Wrap(err, "could not get ai setting")
	}

	// Prepare the custom org instructions
	customOrgInstructions := getOrganizationCustomInstructions(aiSetting)

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
	for _, clientData := range relatedDataPerClient.Data {
		for tableName := range clientData.IngestedData {
			allPresentTables[tableName] = true
		}
	}

	tablesWithLargRowNbs := make(map[string][]string)
	allTables := caseData.dataModel.Tables
	for tableName := range allPresentTables {
		if someClientHasManyRowsForTable(relatedDataPerClient.Data, tableName) {
			tablesWithLargRowNbs[tableName] = allTables[tableName].FieldNames()
		}
	}
	tableNamesWithLargRowNbs := pure_utils.Keys(tablesWithLargRowNbs)

	if len(tablesWithLargRowNbs) > 0 {
		for customerKey, clientData := range relatedDataPerClient.Data {
			objectTables := clientData.IngestedData
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
					relatedDataPerClient.Data[customerKey].IngestedData[tableName] = agent_dto.IngestedDataResult{
						Data:        fieldFilteredObjects,
						ReadOptions: objectTables[tableName].ReadOptions,
					}
				}

			}
		}
	}

	if caseReviewContext.RulesDefinitionsReview == nil {
		organizationDescription := "No description provided"
		if customOrgInstructions.OrgDescription != nil {
			organizationDescription = *customOrgInstructions.OrgDescription
		}

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
		modelCaseReview, promptCaseReview, err := uc.preparePromptWithModel(
			PROMPT_CASE_REVIEW_PATH,
			map[string]any{
				"case_detail":        caseData.case_,
				"case_events":        caseData.events,
				"decisions":          caseData.decisions,
				"data_model_summary": *caseReviewContext.DataModelSummary,
				"pivot_objects":      caseData.pivotData,
				"previous_cases":     relatedDataPerClient,
				"rules_summary":      *caseReviewContext.RulesDefinitionsReview,
				"rule_thresholds":    *caseReviewContext.RuleThresholds,
				"pivot_enrichment":   caseReviewContext.PivotEnrichments,
			},
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not prepare case review request")
		}

		requestCaseReview, err := llmberjack.NewRequest[caseReviewOutput]().
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
		caseReviewContext.CaseReview = &caseReview
	}
	logger.DebugContext(ctx, "================================ Full case review ================================")
	logger.DebugContext(ctx, "Full case review", "response", *caseReviewContext.CaseReview)

	if caseReviewContext.SanityCheck == nil {
		// Finally, sanity check the resulting case review using a judgement prompt
		modelSanityCheck, promptSanityCheck, err := uc.preparePromptWithModel(
			PROMPT_SANITY_CHECK_PATH,
			map[string]any{
				"case_detail":        caseData.case_,
				"case_events":        caseData.events,
				"decisions":          caseData.decisions,
				"data_model_summary": *caseReviewContext.DataModelSummary,
				"pivot_objects":      caseData.pivotData,
				"previous_cases":     relatedDataPerClient,
				"rules_summary":      *caseReviewContext.RulesDefinitionsReview,
				"rule_thresholds":    *caseReviewContext.RuleThresholds,
				"case_review":        *caseReviewContext.CaseReview,
				"pivot_enrichments":  caseReviewContext.PivotEnrichments,
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
	var finalOutput string = caseReviewContext.CaseReview.CaseReview

	instructions, modelForInstruction := uc.getOrganizationInstructionsForPrompt(ctx, customOrgInstructions)

	// If there is no instructions, we don't need to format the output
	if len(instructions) > 0 {
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
			logger.DebugContext(ctx, "could not get custom format", "error", err)
			return nil, errors.Wrap(err, "could not get custom format")
		}
		finalOutput, err = requestCustomFormat.Get(0)
		if err != nil {
			logger.DebugContext(ctx, "could not get custom format", "error", err)
			return nil, errors.Wrap(err, "could not get custom format")
		}
	} else {
		logger.DebugContext(ctx, "No custom instructions for organization, skip this part")
	}

	logger.DebugContext(ctx, "================================ Custom format ================================")
	logger.DebugContext(ctx, "Custom format", "response", finalOutput)

	// Format the proofs
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

	// Can access to Ok and Justification, the nil check is done in the sanity check step
	if caseReviewContext.SanityCheck.Ok {
		return agent_dto.CaseReviewV1{
			Ok:               caseReviewContext.SanityCheck.Ok,
			Output:           finalOutput,
			Proofs:           proofs,
			PivotEnrichments: pivotEnrichments,
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

func (uc *AiAgentUsecase) getCaseDataWithPermissions(ctx context.Context, caseId string) (caseData, agent_dto.CasePivotDataByPivot, error) {
	exec := uc.executorFactory.NewExecutor()
	c, err := uc.repository.GetCaseById(ctx, exec, caseId)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{}, err
	}

	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, c.OrganizationId, false)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "failed to list available inboxes in usecase")
	}
	availableInboxIds := make([]uuid.UUID, len(inboxes))
	for i, inbox := range inboxes {
		availableInboxIds[i] = inbox.Id
	}

	if err := uc.enforceSecurityCase.ReadOrUpdateCase(c.GetMetadata(), availableInboxIds); err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{}, err
	}

	tags, err := uc.repository.ListOrganizationTags(ctx, exec, c.OrganizationId, models.TagTargetCase, false)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve tags for case")
	}
	caseEvents, err := uc.repository.ListCaseEvents(ctx, exec, caseId)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve case events")
	}
	users, err := uc.repository.ListUsers(ctx, exec, &c.OrganizationId)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
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
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve case decisions")
	}

	decicionsWithRulesExec, err := uc.repository.DecisionsWithRuleExecutionsByIds(ctx, exec,
		pure_utils.Map(decisions, func(d models.DecisionWithRulesAndScreeningsBaseInfo) string { return d.DecisionId.String() }))
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve case decisions with rule executions")
	}

	decisionDtos := make([]agent_dto.Decision, len(decicionsWithRulesExec))
	for i, decision := range decicionsWithRulesExec {
		iteration, err := uc.repository.GetScenarioIteration(ctx, exec,
			decision.Decision.ScenarioIterationId.String(), true)
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve scenario for decision %s", decision.DecisionId)
		}
		rules, err := uc.repository.ListRulesByIterationId(ctx, exec,
			decision.Decision.ScenarioIterationId.String())
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve rules for decision %s", decision.DecisionId)
		}
		screenings, err := uc.repository.ListScreeningsForDecision(ctx, exec, decision.DecisionId.String(), true)
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
				"could not retrieve screenings for decision %s", decision.DecisionId)
		}
		decisionDtos[i] = agent_dto.AdaptDecision(decision.Decision, iteration,
			decision.RuleExecutions, rules, screenings)
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, c.OrganizationId, models.DataModelReadOptions{
		IncludeEnums: true, IncludeNavigationOptions: true,
	}, true)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not retrieve data model")
	}

	pivotValues, err := uc.repository.DecisionPivotValuesByCase(ctx, exec, caseId)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{}, err
	}
	pivotObjects, err := uc.ingestedDataReader.ReadPivotObjectsFromValues(ctx, c.OrganizationId, pivotValues)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not read pivot objects from values")
	}
	pivotObjectDtos, err := pure_utils.MapErr(pivotObjects, agent_dto.AdaptPivotObjectDto)
	if err != nil {
		return caseData{}, agent_dto.CasePivotDataByPivot{},
			errors.Wrap(err, "could not adapt pivot objects to DTOs")
	}

	relatedDataPerClient := agent_dto.CasePivotDataByPivot{
		Data: make(map[string]agent_dto.CasePivotObjectData),
	}

	for _, pivotObject := range pivotObjects {
		pivotObjectData := agent_dto.CasePivotObjectData{
			IngestedData: make(map[string]agent_dto.IngestedDataResult, 10),
			RelatedCases: make([]agent_dto.CaseWithDecisions, 0, 10),
		}

		previousCases, err := uc.repository.GetCasesWithPivotValue(ctx, exec,
			c.OrganizationId, pivotObject.PivotValue)
		if err != nil {
			return caseData{}, agent_dto.CasePivotDataByPivot{}, err
		}
		relatedCases := make([]agent_dto.CaseWithDecisions, 0, 10)
		for _, previousCase := range previousCases {
			if previousCase.Id == c.Id {
				// skip the current case, we don't want to include it in the related cases
				continue
			}

			decisions, _, err := uc.repository.DecisionsByCaseIdFromCursor(ctx, exec, models.CaseDecisionsRequest{
				OrgId:  c.OrganizationId,
				CaseId: previousCase.Id,
				Limit:  models.CaseDecisionsPerPage,
			})
			if err != nil {
				return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
					"could not retrieve decisions for previous case %s", previousCase.Id)
			}
			for _, decision := range decisions {
				previousCase.Decisions = append(previousCase.Decisions, decision.Decision)
			}
			events, err := uc.repository.ListCaseEvents(ctx, exec, previousCase.Id)
			if err != nil {
				return caseData{}, agent_dto.CasePivotDataByPivot{}, err
			}
			previousCase.Events = events

			// don't add rule executions because we don't care for previous cases rule exec details
			rc, err := agent_dto.AdaptCaseWithDecisionsDto(
				previousCase,
				tags,
				inboxes,
				users,
				func(scenarioIterationId string) (models.ScenarioIteration, error) {
					return uc.repository.GetScenarioIteration(ctx, exec, scenarioIterationId, true)
				},
				func(decisionId string) ([]models.ScreeningWithMatches, error) {
					return uc.repository.ListScreeningsForDecision(ctx, exec, decisionId, true)
				},
			)
			if err != nil {
				return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
					"could not adapt case with decisions for previous case %s", previousCase.Id)
			}
			relatedCases = append(relatedCases, rc)
		}
		pivotObjectData.RelatedCases = relatedCases

		navigationOptions := dataModel.Tables[pivotObject.PivotObjectName].NavigationOptions
		for _, navOption := range navigationOptions {
			if _, found := pivotObjectData.IngestedData[navOption.TargetTableName]; found {
				// If we already have data for this target table, skip it
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
					return caseData{}, agent_dto.CasePivotDataByPivot{}, errors.Wrapf(err,
						"could not read ingested client objects for %s with value %s",
						pivotObject.PivotObjectName, sourceFieldValueStr)
				}
				pivotObjectData.IngestedData[navOption.TargetTableName] = agent_dto.IngestedDataResult{
					Data:        objects,
					ReadOptions: readOptions,
				}
			}
		}

		relatedDataPerClient.Data[pivotObject.PivotObjectName+"_"+pivotObject.PivotValue] = pivotObjectData
	}

	return caseData{
		case_:          agent_dto.AdaptCaseDto(c, tags, inboxes, users),
		events:         caseEventsDto,
		decisions:      decisionDtos,
		dataModelDto:   agent_dto.AdaptDataModelDto(dataModel),
		dataModel:      dataModel,
		pivotData:      pivotObjectDtos,
		organizationId: c.OrganizationId,
	}, relatedDataPerClient, nil
}

type caseData struct {
	case_          agent_dto.Case
	events         []agent_dto.CaseEvent
	decisions      []agent_dto.Decision
	dataModelDto   agent_dto.DataModel
	dataModel      models.DataModel
	pivotData      []agent_dto.PivotObject
	organizationId string
}

func someClientHasManyRowsForTable(relatedDataPerClient map[string]agent_dto.CasePivotObjectData, tableName string) bool {
	for _, clientData := range relatedDataPerClient {
		if clientData.IngestedData[tableName].Data != nil {
			return len(clientData.IngestedData[tableName].Data) > HIGH_NB_ROWS_THRESHOLD
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

func (uc *AiAgentUsecase) HasAiCaseReviewEnabled(ctx context.Context, orgId string) (bool, error) {
	// Check if the organization has AI case review enabled, fetch the organization and check the flag
	org, err := uc.repository.GetOrganizationById(ctx, uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return false, err
	}
	if !org.AiCaseReviewEnabled {
		return false, nil
	}
	return true, nil
}
