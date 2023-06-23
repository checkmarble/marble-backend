import { useCallback, useMemo, useState } from "react";
import { NewAstNode, type AstNode, type ConstantOptional } from "@/models";
import { MapObjectValues } from "@/MapUtils";
import {
  type ScenariosRepository,
  validateAstExpression,
} from "@/repositories";
import { type LoadingDispatcher, showLoader } from "@/hooks/Loading";

export interface AstExpressionService {
  scenarioRepository: ScenariosRepository;
}

export interface ExpressionViewModel {
  nodes: Map<string, NodeViewModel>;
  rootNode: NodeViewModel;
}

export interface NodeViewModel {
  id: string;
  name: string;
  constant: ConstantOptional;
  children: NodeViewModel[];
  namedChildren: Record<string, NodeViewModel>;
}

function makeExpressionViewModel(node: AstNode): ExpressionViewModel {
  const nodes = new Map<string, NodeViewModel>();

  const makeNodeViewModel = (node: AstNode): NodeViewModel => {
    const newNode: NodeViewModel = {
      id: crypto.randomUUID().toString(),
      name: node.name,
      constant: node.constant,
      children: node.children.map(makeNodeViewModel),
      namedChildren: MapObjectValues(node.namedChildren, makeNodeViewModel),
    };
    nodes.set(newNode.id, newNode);

    return newNode;
  };

  return {
    nodes: nodes,
    rootNode: makeNodeViewModel(node),
  };
}

function adaptAstNodeFromViewModel(vm: ExpressionViewModel): AstNode {
  const adaptAstNode = (node: NodeViewModel): AstNode => {
    return {
      name: node.name,
      constant: node.constant,
      children: node.children.map(adaptAstNode),
      namedChildren: MapObjectValues(node.namedChildren, adaptAstNode),
    };
  };
  return adaptAstNode(vm.rootNode);
}

export function useAstExpressionBuilder(
  service: AstExpressionService,
  pageLoadingDispatcher: LoadingDispatcher
) {
  const [validationErrors, setValidationErrors] = useState<string[]>([]);

  const [expressionViewModel] =
    useState<ExpressionViewModel>(() =>
      makeExpressionViewModel(
        NewAstNode({
          name: ">",
          children: [
            NewAstNode({
              name: "*",
              children: [
                NewAstNode({
                  constant: 2,
                }),
                NewAstNode({
                  constant: 3,
                }),
              ],
            }),
            NewAstNode({
              constant: 10,
            }),
          ],
        })
      )
    );

  const expressionAstNode = useMemo(
    () => adaptAstNodeFromViewModel(expressionViewModel),
    [expressionViewModel]
  );

  const validate = useCallback(async () => {
    const result = await showLoader(
      pageLoadingDispatcher,
      validateAstExpression(
        service.scenarioRepository,
        expressionViewModel.rootNode
      )
    );
    setValidationErrors(result.validationErrors);
  }, [service, expressionViewModel.rootNode, pageLoadingDispatcher]);

  return {
    expressionViewModel,
    expressionAstNode,
    validate,
    validationErrors,
  };
}
