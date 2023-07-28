import { Iteration } from "./Iteration";

export interface Scenario {
  scenarioId: string;
  organizationId: string;
  name: string;
  description: string;
  triggerObjectType: string;
  createdAt: Date;
  liveVersionId: string | null;

  liveIteration: Iteration | null;
}

export interface CreateScenario {
  name: string;
  description: string;
  triggerObjectType: string;
}
