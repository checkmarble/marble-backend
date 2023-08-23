import { useCallback } from "react";
import { LoadingDispatcher } from "@/hooks/Loading";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import type {
  Iteration,
  Scenario,
  ScenarioValidation,
} from "@/models";
import {
  type ScenariosRepository,
  validateIteration,
  fetchScenario,
  fetchIterationsOfScenario,
  fetchIteration,
  fetchScenariosOfOrganization,
} from "@/repositories";

export interface ScenarioService {
  scenariosRepository: ScenariosRepository;
}

export function useScenarios(
  service: ScenarioService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string
) {
  const loadScenarios = useCallback(() => {
    return fetchScenariosOfOrganization(
      service.scenariosRepository,
      organizationId
    );
  }, [service, organizationId]);

  const [scenarios, refreshScenarios] = useSimpleLoader<Scenario[]>(
    loadingDispatcher,
    loadScenarios
  );

  return {
    scenarios,
    refreshScenarios,
  };
}

export function useSingleScenario({
  service,
  loadingDispatcher,
  scenarioId,
  iterationId,
}: {
  service: ScenarioService;
  loadingDispatcher: LoadingDispatcher;
  scenarioId: string;
  iterationId: string | null;
}) {
  const loadScenario = useCallback(async () => {
    const scenario = await fetchScenario(
      service.scenariosRepository,
      scenarioId
    );

    scenario.allIterations = await fetchIterationsOfScenario(
      service.scenariosRepository,
      scenario.organizationId,
      scenarioId
    );

    return scenario;
  }, [service.scenariosRepository, scenarioId]);

  const [scenario, refreshScenario] = useSimpleLoader<Scenario>(
    loadingDispatcher,
    loadScenario
  );

  const loadIteration = useCallback(async () => {
    if (scenario === null || iterationId === null) {
      return null;
    }
    return await fetchIteration(
      service.scenariosRepository,
      scenario.organizationId,
      iterationId
    );
  }, [iterationId, scenario, service.scenariosRepository]);

  const [iteration, refreshIteration] = useSimpleLoader<Iteration>(
    loadingDispatcher,
    loadIteration
  );

  return {
    scenario,
    iteration,
    refreshScenario: async () => {
      await refreshScenario();
      await refreshIteration();
    },
  };
}

export function useIterationValidation({
  service,
  loadingDispatcher,
  iterationId,
}: {
  service: ScenarioService;
  loadingDispatcher: LoadingDispatcher;
  iterationId: string | null;
}) {
  const loadIteration = useCallback(async () => {
    if (iterationId === null) {
      return null;
    }

    return await validateIteration(service.scenariosRepository, iterationId);
  }, [iterationId, service.scenariosRepository]);

  const [iterationValidation] = useSimpleLoader<ScenarioValidation>(
    loadingDispatcher,
    loadIteration
  );

  return {
    iterationValidation,
  };
}
