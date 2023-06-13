export interface Decision {
  decisionId: string;
}

export interface TriggerObject {
  object_id: string;
  updated_at: Date;
  // other fields ?
}

export interface CreateDecision {
  scenario_id: string;
  object_type: string;
  trigger_object: TriggerObject;
}
