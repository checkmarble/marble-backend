import type { ConstantOptional } from "./AstExpression";

export interface EvaluationError {
  error: string;
  message: string;
  argumentIndex: number | null;
  argumentName: string | null;
}

export interface AstNodeEvaluation {
  returnValue: ConstantOptional;
  errors: EvaluationError[] | null;
  children: AstNodeEvaluation[];
  namedChildren: Record<string, AstNodeEvaluation>;
}

export interface ScenarioValidation {
  errors: string[];
  triggerEvaluation: AstNodeEvaluation;
  rulesEvaluations: Record<string, AstNodeEvaluation>;
}
