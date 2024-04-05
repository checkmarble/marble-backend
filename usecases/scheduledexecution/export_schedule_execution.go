package scheduledexecution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type AwsS3Repository interface {
	StoreInBucket(ctx context.Context, bucketName string, key string, body io.Reader) error
}

type ExportScheduleExecution struct {
	AwsS3Repository        AwsS3Repository
	DecisionRepository     repositories.DecisionRepository
	OrganizationRepository repositories.OrganizationRepository
	ExecutorFactory        executor_factory.ExecutorFactory
}

func (exporter *ExportScheduleExecution) ExportScheduledExecutionToS3(ctx context.Context,
	scenario models.Scenario, scheduledExecution models.ScheduledExecution,
) error {
	organization, err := exporter.OrganizationRepository.GetOrganizationById(ctx,
		exporter.ExecutorFactory.NewExecutor(), scheduledExecution.OrganizationId)
	if err != nil {
		return err
	}

	// no s3 configured: no export
	if len(organization.ExportScheduledExecutionS3) == 0 {
		return nil
	}

	numberOfDecision, err := exporter.exportDecisionsToS3(ctx, scheduledExecution,
		organization.ExportScheduledExecutionS3)
	if err != nil {
		return err
	}

	return exporter.exportScenarioToS3(scenario, scheduledExecution,
		organization.ExportScheduledExecutionS3, numberOfDecision)
}

func (exporter *ExportScheduleExecution) exportScenarioToS3(scenario models.Scenario,
	scheduledExecution models.ScheduledExecution, s3Bucket string, numberOfDecision int,
) error {
	filename := fmt.Sprintf("scheduled_scenario_execution_%s.json", scheduledExecution.Id)

	encoded, err := json.Marshal(map[string]any{
		"scheduled_execution_id": scheduledExecution.Id,
		"started_at":             scheduledExecution.StartedAt,
		"scenario": map[string]any{
			"scenario_id": scenario.Id,
			"name":        scenario.Name,
			"description": scenario.Description,
			// "version":     scenario.LiveVersionID,
		},
		"number_of_decisions": numberOfDecision,
	})
	if err != nil {
		return err
	}

	return exporter.AwsS3Repository.StoreInBucket(context.Background(), s3Bucket, filename, bytes.NewReader(encoded))
}

func (exporter *ExportScheduleExecution) exportDecisionsToS3(ctx context.Context,
	scheduledExecution models.ScheduledExecution, s3Bucket string,
) (int, error) {
	pipeReader, pipeWriter := io.Pipe()

	uploadErrorChan := exporter.uploadDecisions(pipeReader, scheduledExecution, s3Bucket)

	// write everything. No need to create a second goroutine, the write can be synchronous.
	number_of_exported_decisions, exportErr :=
		exporter.ExportDecisions(ctx, scheduledExecution.Id, pipeWriter)

	// close the pipe when done
	pipeWriter.Close()

	// wait for upload to finish
	uploadErr := <-uploadErrorChan

	return number_of_exported_decisions, errors.Join(exportErr, uploadErr)
}

func (exporter *ExportScheduleExecution) uploadDecisions(src *io.PipeReader,
	scheduledExecution models.ScheduledExecution, s3Bucket string,
) <-chan error {
	filename := fmt.Sprintf("scheduled_scenario_execution_%s_decisions.ndjson", scheduledExecution.Id)

	// run immediately a goroutine that consume the pipeReader until the pipeWriter is closed
	uploadErrorChan := make(chan error, 1)
	go func() {
		err := exporter.AwsS3Repository.StoreInBucket(context.Background(), s3Bucket, filename, src)

		// Ensure that src is consumed entirely. StoreInBucket can fail without reading everything in src.
		// The goal is to avoid inifinite blocking of PipeWriter.Write
		io.Copy(io.Discard, src)

		uploadErrorChan <- err
	}()
	return uploadErrorChan
}

func (exporter *ExportScheduleExecution) ExportDecisions(ctx context.Context, scheduledExecutionId string, dest io.Writer) (int, error) {
	decisionChan, errorChan := exporter.DecisionRepository.DecisionsOfScheduledExecution(ctx,
		exporter.ExecutorFactory.NewExecutor(), scheduledExecutionId)

	encoder := json.NewEncoder(dest)

	var allErrors []error

	var number_of_exported_decisions int

	for decision := range decisionChan {
		err := encoder.Encode(dto.NewAPIDecisionWithRule(decision, "", false))
		if err != nil {
			allErrors = append(allErrors, err)
		} else {
			number_of_exported_decisions += 1
		}
	}

	// wait for DecisionsOfScheduledExecution to finish
	err := <-errorChan

	return number_of_exported_decisions, errors.Join(append(allErrors, err)...)
}
