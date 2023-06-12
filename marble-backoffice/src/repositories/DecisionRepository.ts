import type { MarbleApi } from "@/infra/MarbleApi";
import type { CreateDecision, Decision } from "@/models";
import {
  adaptDecisionsApiResult,
  adaptSingleDecisionApiResult,
} from "@/models/DecisionDto";

export async function fetchDecisions(
  marbleApi: MarbleApi
): Promise<Decision[]> {
  return adaptDecisionsApiResult(await marbleApi.decisions());
}

export async function postDecision(
  marbleApi: MarbleApi,
  createDecision: CreateDecision
): Promise<Decision> {
  return adaptSingleDecisionApiResult(
    await marbleApi.postDecision(createDecision)
  );
}
