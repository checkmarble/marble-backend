import { Iteration } from "./Iteration";

export interface Scenario {
  scenarioId: string;
  organizationId: string;
  name: string;
  description: string;
  triggerObjectType: string;
  createdAt: Date;
  liveVersionId: string | null;
  allIterations: Iteration[];
}

export interface CreateScenario {
  name: string;
  description: string;
  triggerObjectType: string;
}
