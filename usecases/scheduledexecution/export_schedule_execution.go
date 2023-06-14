package scheduledexecution

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"marble/marble-backend/dto"
	"marble/marble-backend/repositories"
)

type ExportScheduleExecution interface {
	ExportDecisionsToS3(scheduledExecutionId string) error
}

type ExportScheduleExecutionImpl struct {
	AwsS3Repository    repositories.AwsS3Repository
	DecisionRepository repositories.DecisionRepository
}

func (exporter *ExportScheduleExecutionImpl) ExportDecisionsToS3(scheduledExecutionId string) error {
	pipeReader, pipeWriter := io.Pipe()

	// run immediately a goroutine that consume the pipeReader until the pipeWriter is closed
	uploadErrorChan := make(chan error, 1)
	go func() {
		uploadErrorChan <- exporter.AwsS3Repository.StoreInBucket(context.Background(), "bucket", "key", pipeReader)
	}()

	// write everything. No need to create a second goroutine, the write can be synchronous.
	exportErr := exporter.exportDecisions(pipeWriter, scheduledExecutionId)

	// close the pipe when done
	pipeWriter.Close()

	// wait for upload to finish
	uploadErr := <-uploadErrorChan

	return errors.Join(exportErr, uploadErr)
}

func (exporter *ExportScheduleExecutionImpl) exportDecisions(dest *io.PipeWriter, scheduledExecutionId string) error {

	decisionChan, errorChan := exporter.DecisionRepository.DecisionsOfScheduledExecution(scheduledExecutionId)

	encoder := json.NewEncoder(dest)

	// to avoid leak, we must consume the channel until it's closed
	// so let's store all errors
	var allErrors []error

	for decision := range decisionChan {
		err := encoder.Encode(dto.NewAPIDecision(decision))
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// wait for DecisionsOfScheduledExecution to finish
	err := <-errorChan

	return errors.Join(append(allErrors, err)...)
}
