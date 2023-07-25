import type { AstNode } from "./AstExpression";

export interface Rule {
  ruleId: string;
  iterationId: string;
  displayOrder: number;
  name: string;
  description: string;
  formulaAstExpression: AstNode | null;
  scoreModifier: number;
  createdAt: Date;
}

export interface UpdateRule {
  name?: string;
  description?: string;
  expression?: AstNode;
  displayOrder?: number;
  scoreModifier?: number;
}