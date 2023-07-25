import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import type { Iteration } from "./Iteration";

const IterationSchema = yup.object({
  id: yup.string().required(),
  scenarioId: yup.string().required(),
  version: yup.number().defined().nullable(),
  createdAt: yup.date().required(),
  updatedAt: yup.date().required(),

  body: yup.object({
    triggerCondition: yup.string().defined().nullable(),
    //rules
    scoreReviewThreshold: yup.number().defined().nullable(),
    scoreRejectThreshold: yup.number().defined().nullable(),
    batchTriggerSql: yup.string().defined(),
    schedule: yup.string().defined(),
  }),
});

export function adaptIteration(json: unknown): Iteration {
  const dto = adaptDtoWithYup(json, IterationSchema);

  return {
    iterationId: dto.id,
    scenarioId: dto.scenarioId,
    version: dto.version,
    createdAt: dto.createdAt,
    updatedAt: dto.updatedAt,
    triggerCondition: dto.body.triggerCondition || "",
    scoreReviewThreshold: dto.body.scoreReviewThreshold,
    scoreRejectThreshold: dto.body.scoreRejectThreshold,
    batchTriggerSql: dto.body.batchTriggerSql,
    schedule: dto.body.schedule,
  };
}
