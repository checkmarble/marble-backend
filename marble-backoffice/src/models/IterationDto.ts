import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import type { Iteration } from "./Iteration";
import { AstNodeSchemaNullable, adaptAstNode } from "./AstExpressionDto";
import { RuleSchema, adaptRule } from "./RuleDto";
// import {
//   ScenarioValidationSchema,
//   // adaptScenariosValidation,
// } from "./ScenarioValidationDto";

const IterationSchema = yup.object({
  id: yup.string().required(),
  scenarioId: yup.string().required(),
  version: yup.number().defined().nullable(),
  createdAt: yup.date().required(),
  updatedAt: yup.date().required(),

  body: yup.object({
    trigger_condition_ast_expression: AstNodeSchemaNullable,
    rules: yup.array().defined().of(RuleSchema),
    scoreReviewThreshold: yup.number().defined().nullable(),
    scoreRejectThreshold: yup.number().defined().nullable(),
    batchTriggerSql: yup.string().defined(),
    schedule: yup.string().defined(),
  }),
});

export type IterationSchemaDto = yup.InferType<typeof IterationSchema>;

export function adaptIteration(dto: IterationSchemaDto): Iteration {
  return {
    iterationId: dto.id,
    scenarioId: dto.scenarioId,
    version: dto.version,
    createdAt: dto.createdAt,
    updatedAt: dto.updatedAt,
    triggerCondition:
      dto.body.trigger_condition_ast_expression == null
        ? null
        : adaptAstNode(dto.body.trigger_condition_ast_expression),
    scoreReviewThreshold: dto.body.scoreReviewThreshold,
    scoreRejectThreshold: dto.body.scoreRejectThreshold,
    batchTriggerSql: dto.body.batchTriggerSql,
    schedule: dto.body.schedule,
    rules: dto.body.rules.map(adaptRule),
  };
}

export function adaptIterationApiResult(json: unknown): Iteration {
  const dto = adaptDtoWithYup(json, IterationSchema);
  return adaptIteration(dto);
}

export function adaptIterationWithValidationApiResult(json: unknown) {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      iteration: IterationSchema,
    })
  );
  return adaptIteration(dto.iteration);
}

export function adaptListIterationsApiResult(json: unknown): Iteration[] {
  const dto = adaptDtoWithYup(json, yup.array().defined().of(IterationSchema));
  return dto.map(adaptIteration);
}

// export function adaptValidateIterationApiResult(json: unknown) {
//   const dto = adaptDtoWithYup(
//     json,
//     yup.object({
//       scenario_validation: ScenarioValidationSchema,
//     })
//   );
//   return adaptScenariosValidation(dto.scenario_validation);
// }
