import { LoadingDispatcher, showLoader } from "@/hooks/Loading";
// import type { AstNodeEvaluation, ScenarioValidation } from "@/models";
import {
  postScenario,
  postIteration,
  patchIteration,
  // validateIteration,
  postRule,
  updateRule,
  publishIteration,
  OrganizationRepository,
  ScenariosRepository,
} from "@/repositories";
import { exampleTriggerCondition, ExampleRule, demoRules } from "./ExampleAst";

export interface DemoScenarioService {
  organizationRepository: OrganizationRepository;
  scenariosRepository: ScenariosRepository;
}

// function nodeEvaluationErrors(evaluation: AstNodeEvaluation): string[] {
//   return [
//     ...(evaluation.errors ?? []).map((e) => e.message),
//     ...evaluation.children.flatMap(nodeEvaluationErrors),
//     ...Object.values(evaluation.namedChildren).flatMap(nodeEvaluationErrors),
//   ];
// }

// function scenarioValidationErrors(validation: ScenarioValidation): string[] {
//   const errors: string[] = [
//     ...validation.errors.map((e) => `Error: ${e}`),
//     ...nodeEvaluationErrors(validation.triggerEvaluation).map(
//       (e) => `Trigger: ${e}`
//     ),
//     ...Object.values(validation.rulesEvaluations)
//       .map(nodeEvaluationErrors)
//       .flat()
//       .map((e: string) => `Rule: ${e}`),
//   ];

//   return errors;
// }

export function useAddDemoScenarios(
  service: DemoScenarioService,
  loadingDispatcher: LoadingDispatcher,
  organizationId: string,
  refreshScenarios: () => Promise<void>
) {
  return {
    addDemoScenario: async () => {
      const createDemoScenario = async () => {
        // Post a new scenario
        const scenario = await postScenario(
          service.organizationRepository,
          organizationId,
          {
            name: "Demo scenario",
            description: "Demo scenario",
            triggerObjectType: "transactions",
          }
        );

        // post a new iteration
        const iterationId = (
          await postIteration(
            service.scenariosRepository,
            organizationId,
            scenario.scenarioId
          )
        ).iterationId;

        // patch the iteration
        await patchIteration(
          service.scenariosRepository,
          organizationId,
          iterationId,
          {
            scoreReviewThreshold: 20,
            scoreRejectThreshold: 30,
            schedule: "*/10 * * * *",
            triggerCondition: exampleTriggerCondition,
          }
        );

        // const validation = await validateIteration(
        //   service.scenariosRepository,
        //   iterationId
        // );

        // // check for errors in trigger evaluation
        // const errors = nodeEvaluationErrors(validation.triggerEvaluation);
        // if (errors.length > 0) {
        //   throw new Error(errors.join("\n"));
        // }

        const createExampleRule = async (exampleRule: ExampleRule) => {
          const rule = await postRule(
            service.scenariosRepository,
            organizationId,
            iterationId
          );

          await updateRule(
            service.scenariosRepository,
            organizationId,
            rule.ruleId,
            {
              ...exampleRule,
            }
          );

          // const validation = await validateIteration(
          //   service.scenariosRepository,
          //   iterationId
          // );

          // const errors = scenarioValidationErrors(validation);
          // if (errors.length > 0) {
          //   throw new Error(errors.join("\n"));
          // }
        };

        // post a rule
        for (const exampleRule of demoRules) {
          await createExampleRule(exampleRule);
        }

        await publishIteration(
          service.scenariosRepository,
          organizationId,
          iterationId
        );

        await refreshScenarios();
      };

      await showLoader(loadingDispatcher, createDemoScenario());
    },
  };
}
