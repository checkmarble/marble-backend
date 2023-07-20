import { useCallback, useState } from "react";
import {
  type AstNode,
  type ConstantOptional,
  type EditorIdentifiers,
  NewAstNode,
  NoConstant,
  type ConstantType,
  type Scenario,
  type DryRunResult,
} from "@/models";
import { MapObjectValues } from "@/MapUtils";
import {
  type ScenariosRepository,
  validateAstExpression,
  dryRunAstExpression,
  fetchEditorIdentifiers,
  fetchScenario,
} from "@/repositories";
import { type LoadingDispatcher, showLoader } from "@/hooks/Loading";
import { useSimpleLoader } from "@/hooks/SimpleLoader";

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

// function adaptAstNodeFromViewModel(vm: ExpressionViewModel): AstNode {
//   const adaptAstNode = (node: NodeViewModel): AstNode => {
//     return {
//       name: node.name,
//       constant: node.constant ? node.constant : NoConstant,
//       children: node.children.map(adaptAstNode),
//       namedChildren: MapObjectValues(node.namedChildren, adaptAstNode),
//     };
//   };
//   return adaptAstNode(vm.rootNode);
// }

const testAst = NewAstNode({
  name: "IsInList",
  children: [
    NewAstNode({
      name: "DatabaseAccess",
      namedChildren: {
        tableName: NewAstNode({ constant: "transactions" }),
        fieldName: NewAstNode({ constant: 0 }),
        path: NewAstNode({ constant: ["account"] }),
      },
      children: [NewAstNode({ constant: 0 })],
    }),
    NewAstNode({
      name: "CustomListAccess",
      namedChildren: {
        customListId: NewAstNode({
          constant: "d6643d7e-c973-4899-a9a8-805f868ef90a",
        }),
      },
    }),
  ],
});

export function useAstExpressionBuilder(
  service: AstExpressionService,
  scenarioId: string,
  pageLoadingDispatcher: LoadingDispatcher
) {
  const scenarioLoader = useCallback(async () => {
    return showLoader(
      pageLoadingDispatcher,
      fetchScenario(service.scenarioRepository, scenarioId)
    );
  }, [pageLoadingDispatcher, service.scenarioRepository, scenarioId]);

  const [scenario] = useSimpleLoader<Scenario>(
    pageLoadingDispatcher,
    scenarioLoader
  );

  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [dryRunResult, setDryRunResult] = useState<DryRunResult | null>(null);

  const [expressionViewModel, setExpressionViewModel] =
    useState<ExpressionViewModel>(() => makeExpressionViewModel(testAst));

  // const expressionAstNode = useMemo(
  //   () => adaptAstNodeFromViewModel(expressionViewModel),
  //   [expressionViewModel]
  // );
  const expressionAstNode = testAst;

  const editorIdentifiersLoader = useCallback(async () => {
    if (scenario === null) {
      return null;
    }

    return showLoader(
      pageLoadingDispatcher,
      fetchEditorIdentifiers(service.scenarioRepository, scenario.scenarioId)
    );
  }, [pageLoadingDispatcher, scenario, service.scenarioRepository]);

  const [identifiers] = useSimpleLoader<EditorIdentifiers>(
    pageLoadingDispatcher,
    editorIdentifiersLoader
  );

  const validate = useCallback(async () => {
    if (scenario === null) {
      return null;
    }

    const result = await showLoader(
      pageLoadingDispatcher,
      validateAstExpression(
        service.scenarioRepository,
        scenario.organizationId,
        expressionAstNode
      )
    );
    setValidationErrors(result.validationErrors);
  }, [
    scenario,
    pageLoadingDispatcher,
    service.scenarioRepository,
    expressionAstNode,
  ]);

  const run = useCallback(async () => {
    if (scenario === null) {
      return null;
    }

    const dryRunResult = await showLoader(
      pageLoadingDispatcher,
      dryRunAstExpression(
        service.scenarioRepository,
        scenario.organizationId,
        expressionAstNode
      )
    );
    setDryRunResult(dryRunResult);
  }, [
    scenario,
    pageLoadingDispatcher,
    service.scenarioRepository,
    expressionAstNode,
  ]);

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
    dryRunResult,
    run,
    identifiers,
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
  replaceOneNode(editor, nodeId, (node: NodeViewModel): NodeViewModel => {
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

export function addAstNodeOperand(editor: ExpressionEditor, nodeId: string) {
  replaceOneNode(editor, nodeId, (node: NodeViewModel): NodeViewModel => {
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
        },
      ],
    };
  });
}
