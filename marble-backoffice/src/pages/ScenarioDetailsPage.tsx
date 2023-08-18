import services from "@/injectServices";
import Container from "@mui/material/Container";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import ListItemButton from "@mui/material/ListItemButton";
import Typography from "@mui/material/Typography";
import { useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { type AstNode, Rule, PageLink } from "@/models";
import Paper from "@mui/material/Paper";
import TextField from "@mui/material/TextField";
import AddIcon from "@mui/icons-material/Add";
import ReactJson from "react-json-view";
import {
  useAstExpressionBuilder,
  setAstNodeName,
  setAstNodeConstant,
  addAstNodeOperand,
  // deleteAstNodeOperand,
  findNodeIdInDom,
  type NodeViewModel,
} from "@/services/AstExpressionService";
import { useCallback } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useSingleScenario } from "@/services";
import {
  AstNodeComponent,
  AstNodeTextComponent,
} from "@/components/AstNodeComponent";

export default function ScenarioDetailsPage() {
  const { scenarioId } = useParams();
  const [searchParams] = useSearchParams();

  if (!scenarioId) {
    throw Error("scenarioId is required");
  }
  // const navigate = useNavigate();

  const [pageLoading, pageLoadingDispatcher] = useLoading();
  const iterationId = searchParams.get("iteration-id");

  const { scenario, iteration } = useSingleScenario({
    service: services().organizationService,
    loadingDispatcher: pageLoadingDispatcher,
    scenarioId,
    iterationId,
  });

  const {
    editor,
    // expressionAstNode,
    identifiers,
  } = useAstExpressionBuilder(
    services().astExpressionService,
    scenarioId,
    pageLoadingDispatcher
  );

  const handleOperatorNameChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const nodeId = findNodeIdInDom(event.target);
      setAstNodeName(editor, nodeId, event.target.value);
    },
    [editor]
  );

  const handleAstNodeConstantChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const nodeId = findNodeIdInDom(event.target);
      setAstNodeConstant(editor, nodeId, event.target.value);
    },
    [editor]
  );

  const handleAddAstNodeOperand = useCallback(
    (event: React.MouseEvent<HTMLElement>) => {
      const nodeId = findNodeIdInDom(event.currentTarget);
      addAstNodeOperand(editor, nodeId);
    },
    [editor]
  );

  // const handleDeleteAstNode = useCallback(
  //   (event: React.MouseEvent<HTMLElement>) => {
  //     const nodeId = findNodeIdInDom(event.currentTarget);
  //     deleteAstNodeOperand(editor, nodeId);
  //   },
  //   [editor]
  // );

  //@ts-ignore
  const nodeEditor: NodeEditor = {
    handleOperatorNameChange,
    handleAstNodeConstantChange,
    handleAddAstNodeOperand,
    // handleDeleteAstNode,
  };

  const navigate = useNavigate();

  const handleEditTrigger = useCallback(() => {
    if (iterationId === null) {
      throw Error("iterationId is required");
    }
    navigate(PageLink.editTrigger(scenarioId, iterationId));
  }, [iterationId, navigate, scenarioId]);

  const handleEditRule = useCallback(
    (ruleId: string) => {
      if (iterationId === null) {
        throw Error("iterationId is required");
      }
      navigate(PageLink.editRule(scenarioId, iterationId, ruleId));
    },
    [iterationId, navigate, scenarioId]
  );

  const handleIterationClick = useCallback(
    (iterationId: string) => {
      navigate(PageLink.scenarioDetailsPage(scenarioId, iterationId));
    },
    [navigate, scenarioId]
  );

  const iterationEditable = iteration?.version === null || false;

  return (
    <>
      <DelayedLinearProgress loading={pageLoading} />
      <Container
        sx={{
          maxWidth: "md",
        }}
      >
        <Stack direction="column" spacing={2}>
          {scenario && iterationId == null && (
            <>
              <Typography variant="h5">
                {scenario.allIterations.length} iterations
              </Typography>
              <List>
                {scenario.allIterations.map((iteration) => (
                  <ListItem>
                    <ListItemButton
                      onClick={() =>
                        handleIterationClick(iteration.iterationId)
                      }
                    >
                      <ListItemText>
                        {iteration.version === null
                          ? "Draft Iteration"
                          : `Live Iteration version ${iteration.version}`}
                      </ListItemText>
                    </ListItemButton>
                  </ListItem>
                ))}
              </List>
            </>
          )}

          {/* Iteration */}
          {iteration && (
            <>
              <Typography variant="h5">
                {iteration.version ? (
                  <>Live Iteration version {iteration.version}</>
                ) : (
                  <>Draft Iteration</>
                )}
              </Typography>
              <TriggerCondition
                onEditTrigger={iterationEditable ? handleEditTrigger : null}
                triggerCondition={iteration.triggerCondition}
              />
              {iteration.rules.map((rule) => (
                <RuleComponent
                  onEditRule={
                    iterationEditable ? () => handleEditRule(rule.ruleId) : null
                  }
                  key={rule.ruleId}
                  rule={rule}
                />
              ))}
            </>
          )}
          {/* {iteration != null && (
            <>

            </>
          )} */}
          {/* <Typography variant="h5">Expression Editor</Typography>

          <AstEditor
            editor={nodeEditor}
            node={editor.expressionViewModel.rootNode}
          /> */}
          {identifiers && (
            <>
              <Typography variant="h5">Builder Identifiers</Typography>
              <Paper sx={{ minWidth: "100%", p: 2, fontSize: "0.8em" }}>
                <ReactJson
                  src={identifiers}
                  collapsed={true}
                  theme={"rjv-default"}
                />
              </Paper>
            </>
          )}
          {/* <Typography variant="h5">Simple rendering of the AST</Typography>
          <AstNodeComponent node={expressionAstNode} evaluation={dryRunResult} /> */}
        </Stack>
      </Container>
    </>
  );
}

function TriggerCondition({
  onEditTrigger,
  triggerCondition,
}: {
  onEditTrigger: (() => void) | null;
  triggerCondition: AstNode | null;
}) {
  return (
    <>
      <Typography variant="h6">Trigger condition</Typography>
      {triggerCondition === null ? (
        <>No trigger condition</>
      ) : (
        <>
          {onEditTrigger && (
            <Button onClick={onEditTrigger} color="secondary">
              Edit trigger
            </Button>
          )}
          <AstNodeTextComponent node={triggerCondition} />
          <AstNodeComponent node={triggerCondition} />
        </>
      )}
    </>
  );
}

function RuleComponent({
  rule,
  onEditRule,
}: {
  rule: Rule;
  onEditRule: (() => void) | null;
}) {
  return (
    <>
      <Typography variant="h6">Rule {rule.name}</Typography>
      <Typography variant="subtitle1">Rule {rule.description}</Typography>
      {rule.formulaAstExpression === null ? (
        <>No formula</>
      ) : (
        <>
          {onEditRule && (
            <Button onClick={onEditRule} color="secondary">
              Edit Rule
            </Button>
          )}
          <AstNodeTextComponent node={rule.formulaAstExpression} />
          <AstNodeComponent node={rule.formulaAstExpression} />
        </>
      )}
    </>
  );
}

interface NodeEditor {
  handleOperatorNameChange: (
    event: React.ChangeEvent<HTMLInputElement>
  ) => void;
  handleAstNodeConstantChange: (
    event: React.ChangeEvent<HTMLInputElement>
  ) => void;
  handleAddAstNodeOperand: (event: React.MouseEvent<HTMLElement>) => void;
  // handleDeleteAstNode: (event: React.MouseEvent<HTMLElement>) => void;
}

//@ts-ignore
function AstEditor({
  editor,
  node,
}: {
  editor: NodeEditor;
  node: NodeViewModel;
}) {
  return (
    <Paper
      sx={{
        margin: 2,
        padding: 1,
        border: 1,
      }}
      data-node-id={node.id}
    >
      <Stack
        direction="row"
        spacing={2}
        sx={{
          alignItems: "baseline",
        }}
      >
        <TextField
          sx={{ mr: 2 }}
          autoFocus
          margin="dense"
          label="Function"
          variant="standard"
          value={node.name}
          onChange={editor.handleOperatorNameChange}
        />
        <TextField
          sx={{ mb: 1 }}
          autoFocus
          margin="dense"
          label="Constant"
          variant="standard"
          value={node.constant}
          onChange={editor.handleAstNodeConstantChange}
        />
        {/* <Button onClick={editor.handleDeleteAstNode}>Delete</Button> */}
      </Stack>
      {node.children.map((child) => (
        <AstEditor key={child.id} editor={editor} node={child} />
      ))}
      <Button onClick={editor.handleAddAstNodeOperand}>
        <AddIcon />
        Operand
      </Button>
      {Object.entries(node.namedChildren).map(([name, child]) => (
        <>
          {name} <AstEditor key={child.id} editor={editor} node={child} />
        </>
      ))}

      {/* {node.name && <Typography variant="subtitle1">{node.name}</Typography>} */}
    </Paper>
  );
}
