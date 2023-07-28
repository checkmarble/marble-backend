import * as yup from "yup";
import { adaptDtoWithYup } from "@/infra/adaptDtoWithYup";
import type { Rule } from "./Rule";
import { adaptAstNode, AstNodeSchemaNullable } from "./AstExpressionDto";

// {
//     "id": "13617a88-a382-4985-9783-49053d0e8b3b",
//     "scenarioIterationId": "13617a88-7b50-4467-8fd3-f9b464dd7625",
//     "displayOrder": 0,
//     "name": "",
//     "description": "",
//     "formula": null,
//     "formula_ast_expression": null,
//     "scoreModifier": 0,
//     "createdAt": "2023-07-25T17:42:12.08125+02:00"
// }

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
  const dto = adaptDtoWithYup(json, RuleSchema);
  return adaptRule(dto);
}
