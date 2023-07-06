export interface Scenario {
  scenarioId: string;
  organizationId: string;
  name: string;
  description: string;
  triggerObjectType: string;
  createdAt: Date;
  liveVersionId: string | null;
}
