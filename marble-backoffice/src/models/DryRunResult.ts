
import { ConstantOptional } from "./AstExpression";

export interface DryRunResult {
  result: ConstantOptional;
  runtimeError: string;
}
