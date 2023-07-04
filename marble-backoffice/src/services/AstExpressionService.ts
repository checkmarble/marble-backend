import { useCallback, useMemo, useState } from "react";
import {
  NewAstNode,
  type AstNode,
  type ConstantOptional,
  NoConstant,
  ConstantType,
} from "@/models";
import { MapObjectValues } from "@/MapUtils";
import {
  type ScenariosRepository,
  validateAstExpression,
  runAstExpression,
} from "@/repositories";
import { type LoadingDispatcher, showLoader } from "@/hooks/Loading";

export interface AstExpressionService {
  scenarioRepository: ScenariosRepository;
}

export interface ExpressionEditor {
  expressionViewModel: ExpressionViewModel;
  setExpressionViewModel: (vm: ExpressionViewModel) => void;
  pageLoadingDispatcher: LoadingDispatcher;
}

export interface ExpressionViewModel {
  rootNode: NodeViewModel;
}

export interface NodeViewModel {
  id: string;
  name: string;
  constant: ConstantType;
  children: NodeViewModel[];
  namedChildren: Record<string, NodeViewModel>;
}

function jsonifyConstant(constant: ConstantOptional): ConstantType {
  if (constant === NoConstant) {
    return "";
  } else {
    return constant;
  }
}

function makeExpressionViewModel(node: AstNode): ExpressionViewModel {
  // const nodes = new Map<string, NodeViewModel>();

  const makeNodeViewModel = (node: AstNode): NodeViewModel => {
    const newNode: NodeViewModel = {
      id: crypto.randomUUID().toString(),
      name: node.name,
      constant: jsonifyConstant(node.constant),
      children: node.children.map(makeNodeViewModel),
      namedChildren: MapObjectValues(node.namedChildren, makeNodeViewModel),
    };
    // nodes.set(newNode.id, newNode);

    return newNode;
  };

  return {
    // nodes: nodes,
    rootNode: makeNodeViewModel(node),
  };
}

function adaptAstNodeFromViewModel(vm: ExpressionViewModel): AstNode {
  const adaptAstNode = (node: NodeViewModel): AstNode => {
    return {
      name: node.name,
      constant: node.constant ? node.constant : NoConstant,
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

  const [expressionViewModel, setExpressionViewModel] =
    useState<ExpressionViewModel>(() =>
      makeExpressionViewModel(
        NewAstNode({
          name: "IsInList",
          children: [
            NewAstNode({
              name: "DatabaseAccess",
              namedChildren: {
                tableName: NewAstNode({ constant: "transactions"}), 
                fieldName: NewAstNode({ constant: "name"}),
                path: NewAstNode({ constant: ["account"]}),
              }}),
              NewAstNode({
                name: "CustomListAccess",
                namedChildren: {
                  customListId: NewAstNode({ constant: "0d968583-20da-4199-bf4a-020566a36251"}), 
                }}),
            ],
        }))
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
        expressionAstNode
      )
    );
    setValidationErrors(result.validationErrors);
  }, [service, expressionAstNode, pageLoadingDispatcher]);

  const run = useCallback(async () => {
    await showLoader(
      pageLoadingDispatcher,
      runAstExpression(
        service.scenarioRepository,
        expressionAstNode
      )
    );
  }, [service, expressionAstNode, pageLoadingDispatcher]);


  const editor: ExpressionEditor = {
    expressionViewModel,
    setExpressionViewModel,
    pageLoadingDispatcher,
  };

  return {
    editor,
    expressionAstNode,
    validate,
    validationErrors,
    run,
  };
}

function replaceOneNode(
  editor: ExpressionEditor,
  nodeId: string,
  fn: (node: NodeViewModel) => NodeViewModel
) {
  function replaceNode(node: NodeViewModel): NodeViewModel {
    if (node.id === nodeId) {
      return fn(node);
    }

    // Possible optimization: copy just the parent of the target node.
    // but I want to avoid early optimization.
    const children = node.children.map(replaceNode);
    const namedChildren = MapObjectValues(node.namedChildren, replaceNode);
    return {
      ...node,
      children,
      namedChildren,
    };
  }

  editor.setExpressionViewModel({
    ...editor.expressionViewModel,
    rootNode: replaceNode(editor.expressionViewModel.rootNode),
  });
}

export function setAstNodeName(
  editor: ExpressionEditor,
  nodeId: string,
  newName: string
) {
  replaceOneNode(editor, nodeId, (node) => ({
    ...node,
    name: newName,
    constant: "",
  }));
}

export function setAstNodeConstant(
  editor: ExpressionEditor,
  nodeId: string,
  newConstant: string
) {
  replaceOneNode(editor, nodeId, (node: NodeViewModel) : NodeViewModel => {
    return {
      ...node,
      name: "",
      constant: newConstant,
      children: [],
      namedChildren: {},
    };
  });
}

export function findNodeIdInDom(startNode: HTMLElement | null): string {
  const key = "nodeId";
  let node: HTMLElement | null = startNode;
  while (node !== null) {
    const payload = node.dataset[key];
    if (payload) {
      return payload;
    }
    node = node.parentElement;
  }
  throw Error("nodeId is missing");
}


export function addAstNodeOperand(
  editor: ExpressionEditor,
  nodeId : string,
) {
  replaceOneNode(editor, nodeId, (node: NodeViewModel) : NodeViewModel => {
    return {
      ...node,
      constant: "",
      children: [
        ...node.children,
        {
          id: crypto.randomUUID().toString(),
          name: "",
          constant: "",
          children: [],
          namedChildren: {},
        }
      ],
    };
  });
}
