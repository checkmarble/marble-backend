import { type MarbleApi } from "@/infra/MarbleApi";
import { type LoadingDispatcher, showLoader } from "@/hooks/Loading";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { useCallback, useState } from "react";
import { fetchDecisions, postDecision } from "@/repositories";
import type { Decision } from "@/models";

export function useDecisions(
  marbleApi: MarbleApi | null,
  loadingDispatcher: LoadingDispatcher
) {
  const decisionsLoader = useCallback(async (): Promise<Decision[] | null> => {
    if (marbleApi === null) {
      return null; // MarbleApi not initialized yet
    }
    return fetchDecisions(marbleApi);
  }, [marbleApi]);

  const [decisions, refreshDecisions] = useSimpleLoader(
    loadingDispatcher,
    decisionsLoader
  );

  return {
    decisions,
    refreshDecisions,
  };
}

export interface CreateScenarioViewModel {
  scenarioId: string;
}

export function useCreateDecision(
  marbleApi: MarbleApi | null,
  loadingDispatcher: LoadingDispatcher,
  refreshDecisions: () => void
) {
  const [createScenarioViewModel, setCreateScenarioViewModel] =
    useState<CreateScenarioViewModel>({
      scenarioId: "",
    });

  const createDecisionformValid = !!createScenarioViewModel.scenarioId;

  const createDecision = useCallback(async (): Promise<Decision | null> => {
    if (marbleApi === null) {
      return null; // MarbleApi not initialized yet
    }

    const triggerObject = {
      object_id: "transaction_c",
      account_id: "account-a-id",
      direction: "payout",
      status: "pending",
      bic_country: "FR",
      amount: 100,
      updated_at: new Date(),
    };

    return showLoader(
      loadingDispatcher,
      postDecision(marbleApi, {
        scenario_id: createScenarioViewModel.scenarioId,
        object_type: "transactions",
        trigger_object: triggerObject,
      })
    );
  }, [marbleApi, loadingDispatcher, createScenarioViewModel]);

  return {
    createDecision,
    refreshDecisions,
    createScenarioViewModel,
    setCreateScenarioViewModel,
    createDecisionformValid,
  };
}
