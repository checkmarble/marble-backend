import { useEffect, useState } from "react";
// import debounce from "@mui/material/utils/debounce";
import { showLoader, type LoadingDispatcher } from "@/hooks/Loading";
import {
  updateRule,
  type ScenariosRepository,
  patchIteration,
  validateIteration,
} from "@/repositories";
import type { AstNode, AstNodeEvaluation, EvaluationError, Iteration, Scenario } from "@/models";
import {
  adaptAstNodeDto,
  adaptLitteralAstNode,
} from "@/models/AstExpressionDto";

export interface AstEditorService {
  scenariosRepository: ScenariosRepository;
}

export function useAstEditor(
  service: AstEditorService,
  loadingDispatcher: LoadingDispatcher,
  scenario: Scenario | null,
  iteration: Iteration | null,
  ruleId: string | null
) {
  const [astText, setAstText] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string>("");

  useEffect(() => {
    if (astText !== null || iteration === null) {
      return;
    }

    const triggerAst = iteration?.triggerCondition;
    const rule = iteration?.rules.find((r) => r.ruleId === ruleId);
    const ruleAst = rule?.formulaAstExpression;
    setAstText(
      ruleAst
        ? stringifyAst(ruleAst)
        : triggerAst
        ? stringifyAst(triggerAst)
        : ""
    );
  }, [astText, ruleId, iteration]);

  // useEffect(() => {
  //   debounce()
  // }, [])

  useEffect(() => {
    if (astText === null) {
      return;
    }
    if (scenario === null || iteration === null) {
      throw Error("can't update rule/trigger, Scenario or Iteration is null");
    }

    const { astNode, errorMessage } = validateAstText(astText);
    if (errorMessage) {
      setErrorMessage(errorMessage);
    }

    if (astNode) {
      showLoader(
        loadingDispatcher,
        (async () => {
          if (ruleId === null) {
            await patchIteration(
              service.scenariosRepository,
              scenario.organizationId,
              iteration.iterationId,
              {
                triggerCondition: astNode,
              }
            );
          } else {
            await updateRule(
              service.scenariosRepository,
              scenario.organizationId,
              ruleId,
              {
                formula: astNode,
              }
            );
          }
          const validation = await validateIteration(
            service.scenariosRepository,
            iteration.iterationId
          );

          const flattenNodeEvaluation = (
            validation: AstNodeEvaluation
          ): AstNodeEvaluation[] => {
            return [
              validation,
              ...validation.children.map(flattenNodeEvaluation).flat(),
              ...Object.values(validation.namedChildren)
                .map(flattenNodeEvaluation)
                .flat(),
            ];
          };
          const numberofErrors =
            validation.errors.length +
            flattenNodeEvaluation(validation.triggerEvaluation).length +
            Object.values(validation.rulesEvaluations)
              .map(flattenNodeEvaluation)
              .flat().length;
          setErrorMessage(`${numberofErrors} validation errors`);
        })()
      );
    }
  }, [service, scenario, iteration, loadingDispatcher, ruleId, astText]);

  return {
    astText,
    setAstText: setAstText,
    errorMessage,
  };
}

function validateAstText(astText: string): {
  astNode?: AstNode;
  errorMessage?: string;
} {
  try {
    const parsedJson = JSON.parse(astText);
    const astNode = adaptLitteralAstNode(parsedJson);

    return { astNode: astNode };
  } catch (e) {
    if (e instanceof Error) {
      return { errorMessage: `${e}` };
    }
    throw e;
  }
}

function stringifyAst(ast: AstNode) {
  return JSON.stringify(adaptAstNodeDto(ast), null, 2);
}

// return just an array of error from a recursive evaluation
export function flattenNodeEvaluationErrors(
  evaluation: AstNodeEvaluation
): EvaluationError[] {
  return [
    ...(evaluation.errors ?? []),
    ...evaluation.children.map(flattenNodeEvaluationErrors).flat(),
    ...Object.values(evaluation.namedChildren)
      .map(flattenNodeEvaluationErrors)
      .flat(),
  ];
}
