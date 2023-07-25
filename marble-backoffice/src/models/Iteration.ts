export interface Iteration {
  iterationId: string;
  scenarioId: string;
  version: number | null;
  createdAt: Date;
  updatedAt: Date;
  triggerCondition: string;
  // rules : Rule[];
  scoreReviewThreshold: number | null;
  scoreRejectThreshold: number | null;
  batchTriggerSql: string;
  schedule: string;
}

export interface UpdateIteration {
  // triggerCondition: dto.body.triggerCondition || "",
  scoreReviewThreshold?: number;
  scoreRejectThreshold?: number;
  schedule?: string;
  batchTriggerSql?: string;
}
