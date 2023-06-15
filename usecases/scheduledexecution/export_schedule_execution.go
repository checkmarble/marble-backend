package scheduledexecution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"marble/marble-backend/dto"
	"marble/marble-backend/repositories"
)

type ExportScheduleExecution interface {
	ExportDecisionsToS3() error
}

type ExportScheduleExecutionImpl struct {
	AwsS3Repository    repositories.AwsS3Repository
	DecisionRepository repositories.DecisionRepository
}

func (exporter *ExportScheduleExecutionImpl) ExportDecisionsToS3() error {
	scheduledExecutionId := "{execution_id}"

	pipeReader, pipeWriter := io.Pipe()

	uploadErrorChan := exporter.uploadDecisions(pipeReader, scheduledExecutionId)

	// write everything. No need to create a second goroutine, the write can be synchronous.
	exportErr := exporter.exportDecisions(pipeWriter, scheduledExecutionId)

	// close the pipe when done
	pipeWriter.Close()

	// wait for upload to finish
	uploadErr := <-uploadErrorChan

	return errors.Join(exportErr, uploadErr)
}

func (exporter *ExportScheduleExecutionImpl) uploadDecisions(src *io.PipeReader, scheduledExecutionId string) <-chan error {
	bucket := "marble-backend-export-scheduled-execution-test"

	filename := fmt.Sprintf("scheduled_scenario_execution_%s_decisions.ndjson", scheduledExecutionId)

	// run immediately a goroutine that consume the pipeReader until the pipeWriter is closed
	uploadErrorChan := make(chan error, 1)
	go func() {
		err := exporter.AwsS3Repository.StoreInBucket(context.Background(), bucket, filename, src)

		// Ensure that src is consumed entirely. StoreInBucket can fail without reading everything in src.
		// The goal is to avoid inifinite blocking of PipeWriter.Write
		io.Copy(io.Discard, src)

		uploadErrorChan <- err
	}()
	return uploadErrorChan
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
