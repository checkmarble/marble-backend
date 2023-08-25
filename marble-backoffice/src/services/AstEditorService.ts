import { useEffect, useState } from "react";
// import debounce from "@mui/material/utils/debounce";
import { showLoader, type LoadingDispatcher } from "@/hooks/Loading";
import {
  updateRule,
  type ScenariosRepository,
  patchIteration,
  validateIteration,
} from "@/repositories";
import type {
  AstNode,
  AstNodeEvaluation,
  EvaluationError,
  Iteration,
  Scenario,
} from "@/models";
import {
  adaptAstNodeDto,
  adaptLitteralAstNode,
} from "@/models/AstExpressionDto";
import { HttpError } from "@/infra/fetchUtils";

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
  const [errorMessages, setErrorMessages] = useState<string[]>([]);
  const [validation, setValidation] = useState<AstNodeEvaluation | null>(null);

  // set initial value of astText
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

  // save rule/trigger and validate
  useEffect(() => {
    if (astText === null) {
      return;
    }
    if (scenario === null || iteration === null) {
      throw Error("can't update rule/trigger, Scenario or Iteration is null");
    }

    const { astNode, errorMessage } = validateAstText(astText);
    if (errorMessage) {
      setErrorMessages([errorMessage]);
    }

    if (!astNode) {
      return;
    }

    showLoader(
      loadingDispatcher,
      (async () => {
        try {
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
        } catch (e) {
          if (e instanceof HttpError) {
            if (e.statusCode >= 400 && e.statusCode < 500) {
              const message = (await e.response.text()).split("\n")[0]
              setErrorMessages([message]);
            }
          }
          throw e;
        }

        const validation = await validateIteration(
          service.scenariosRepository,
          iteration.iterationId
        );

        setValidation(
          ruleId === null
            ? validation.triggerEvaluation
            : validation.rulesEvaluations[ruleId]
        );
      })()
    );
  }, [service, scenario, iteration, loadingDispatcher, ruleId, astText]);

  // update error message
  useEffect(() => {
    if (validation === null) {
      return;
    }

    setErrorMessages(
      flattenNodeEvaluationErrors(validation).map((e) => e.message)
    );
  }, [validation]);

  return {
    astText,
    setAstText: setAstText,
    errorMessages,
    validation,
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
