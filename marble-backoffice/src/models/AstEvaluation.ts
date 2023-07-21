import type { ConstantOptional } from "./AstExpression";

export interface AstNodeEvaluation {
  returnValue: ConstantOptional;
  evaluationError: string;
  children: AstNodeEvaluation[];
  namedChildren: Record<string, AstNodeEvaluation>;
}
