import * as yup from "yup";
import { type ConstantType } from "./AstExpression";
import {
  ConstantOptionalSchema,
  adaptConstantOptional,
} from "./AstExpressionDto";
import type {
  // ScenarioValidation,
  AstNodeEvaluation,
  EvaluationError,
} from "./ScenarioValidation";
import { MapObjectValues } from "@/MapUtils";

export const EvaluationErrorSchema = yup.object({
  error: yup.string().defined(),
  message: yup.string().defined(),
  argument_index: yup.number(),
  argument_name: yup.string(),
});

type EvaluationErrorDto = yup.InferType<typeof EvaluationErrorSchema>;

// Yup can't infer typescript type from recursive schema, let's declare it manually
export interface AstNodeEvaluationDto {
  return_value?: ConstantType;
  errors: EvaluationErrorDto[] | null;
  children?: AstNodeEvaluationDto[];
  named_children?: { [key: string]: AstNodeEvaluationDto };
}

export const AstNodeEvaluationSchema = yup.object({
  return_value: ConstantOptionalSchema,
  errors: yup.array().nullable().of(EvaluationErrorSchema),
  children: yup
    .array()
    .of(yup.lazy(() => AstNodeEvaluationSchema.default(null)))
    .optional(),
  named_children: lazyObjectOf(() => AstNodeEvaluationSchema),
}) as yup.Schema<AstNodeEvaluationDto>; // Can't use lazy schema as array().of argument in TypeScript: https://github.com/jquense/yup/issues/1190

function lazyObjectOf<Schema extends yup.Schema>(schema: () => Schema) {
  const weirdType = yup.lazy((obj) => {
    return yup.object(
      MapObjectValues(obj || {}, () => {
        return schema().default(undefined);
      })
    );
  });
  return weirdType as unknown as yup.Schema<
    Record<string, yup.InferType<Schema>>
  >;
}

export const ScenarioValidationSchema = yup.object({
  decision: yup.array().of(yup.string().defined()),
  trigger: AstNodeEvaluationSchema,
  rules: lazyObjectOf(() => AstNodeEvaluationSchema),
});

// type ScenarioValidationDto = yup.InferType<typeof ScenarioValidationSchema>;

export function adaptEvaluationError(dto: EvaluationErrorDto): EvaluationError {
  return {
    error: dto.error,
    message: dto.message,
    argumentIndex: dto.argument_index ?? null,
    argumentName: dto.argument_name ?? null,
  };
}

export function adaptNodeEvaluation(
  dto: AstNodeEvaluationDto
): AstNodeEvaluation {
  return {
    returnValue: adaptConstantOptional(dto.return_value),
    errors: dto.errors === null ? null : dto.errors.map(adaptEvaluationError),
    children: (dto.children || []).map(adaptNodeEvaluation),
    namedChildren: MapObjectValues(
      dto.named_children || {},
      adaptNodeEvaluation
    ),
  };
}

// export function adaptScenariosValidation(
//   dto: ScenarioValidationDto
// ): ScenarioValidation {
//   return {
//     errors: dto.errors,
//     triggerEvaluation: adaptNodeEvaluation(dto.trigger_evaluation),
//     rulesEvaluations: MapObjectValues(
//       dto.rules_evaluations,
//       adaptNodeEvaluation
//     ),
//   };
// }
