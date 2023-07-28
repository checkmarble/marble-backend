import { type AstNode } from "./AstExpression";
import { Rule } from "./Rule";

export interface Iteration {
  iterationId: string;
  scenarioId: string;
  version: number | null;
  createdAt: Date;
  updatedAt: Date;
  triggerCondition: AstNode | null;
  rules : Rule[];
  scoreReviewThreshold: number | null;
  scoreRejectThreshold: number | null;
  batchTriggerSql: string;
  schedule: string;
}

export interface UpdateIteration {
  triggerCondition?: AstNode;
  scoreReviewThreshold?: number;
  scoreRejectThreshold?: number;
  schedule?: string;
  batchTriggerSql?: string;
}
