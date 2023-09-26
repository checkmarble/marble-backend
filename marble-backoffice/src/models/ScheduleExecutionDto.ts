import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import { type ScheduleExecution } from "./ScheduleExecution";

const ScheduleExecutionSchema = yup.object({
  id: yup.string().required(),
  scenario_iteration_id: yup.string().required(),
  status: yup.string().defined(),
  started_at: yup.date().defined(),
  finished_at: yup.date().defined().nullable(),
  number_of_created_decisions: yup.number().defined(),
});

export function adaptScheduleExecution(
  dto: yup.InferType<typeof ScheduleExecutionSchema>
): ScheduleExecution {
  return {
    id: dto.id,
    scenario_iteration_id: dto.scenario_iteration_id,
    status: dto.status,
    started_at: dto.started_at,
    finished_at: dto.finished_at,
    numberOfCreatedDecisions: dto.number_of_created_decisions,
  };
}

export function adaptScheduleExecutionApiResult(
  json: unknown
): ScheduleExecution[] {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      scheduled_executions: yup.array().defined().of(ScheduleExecutionSchema),
    })
  );
  return dto.scheduled_executions.map(adaptScheduleExecution);
}
