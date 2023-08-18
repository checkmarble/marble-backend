import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import type { Rule } from "./Rule";
import { adaptAstNode, AstNodeSchemaNullable } from "./AstExpressionDto";

export const RuleSchema = yup.object({
  id: yup.string().required(),
  scenarioIterationId: yup.string().required(),
  displayOrder: yup.number().defined(),
  name: yup.string().defined(),
  description: yup.string().defined(),
  formula_ast_expression: AstNodeSchemaNullable,
  scoreModifier: yup.number().defined(),
  createdAt: yup.date().required(),
});

export type RuleSchemaDto = yup.InferType<typeof RuleSchema>;

export function adaptRule(dto: RuleSchemaDto): Rule {
  return {
    ruleId: dto.id,
    iterationId: dto.scenarioIterationId,
    displayOrder: dto.displayOrder,
    name: dto.name,
    description: dto.description,
    formulaAstExpression:
      dto.formula_ast_expression === null
        ? null
        : adaptAstNode(dto.formula_ast_expression),
    scoreModifier: dto.scoreModifier,
    createdAt: dto.createdAt,
  };
}

export function adaptRuleApiResult(json: unknown): Rule {
  const dto = adaptDtoWithYup(json, yup.object({ rule: RuleSchema }));
  return adaptRule(dto.rule);
}

export function adaptUpdateRuleApiResult(json: unknown): Rule {
  const dto = adaptDtoWithYup(
    json,
    yup.object({
      rule: RuleSchema,
    })
  );
  return adaptRule(dto.rule);
}
