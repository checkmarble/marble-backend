import type { ConstantOptional } from "./AstExpression";

export interface AstNodeEvaluation {
  returnValue: ConstantOptional;
  evaluationError: string;
  children: AstNodeEvaluation[];
  namedChildren: Record<string, AstNodeEvaluation>;
}

export interface ScenarioValidation {
  errors: string[];
  triggerEvaluation: AstNodeEvaluation;
  rulesEvaluations: AstNodeEvaluation[];
}
