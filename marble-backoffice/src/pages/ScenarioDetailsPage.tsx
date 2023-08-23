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
import { type AstNode, Rule, PageLink, AstNodeEvaluation } from "@/models";
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
import { useIterationValidation, useSingleScenario } from "@/services";
import {
  AstNodeComponent,
  AstNodeTextComponent,
} from "@/components/AstNodeComponent";
import Box from "@mui/material/Box";
import ListItemAvatar from "@mui/material/ListItemAvatar";
import Avatar from "@mui/material/Avatar";
import MovieIcon from "@mui/icons-material/Movie";

export default function ScenarioDetailsPage() {
  const { scenarioId } = useParams();
  const [searchParams] = useSearchParams();
  const iterationId = searchParams.get("iteration-id");

  if (!scenarioId) {
    throw Error("scenarioId is required");
  }
  // const navigate = useNavigate();

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { scenario, iteration } = useSingleScenario({
    service: services().scenarioService,
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

  const navigate = useNavigate();

  //@ts-ignore
  const nodeEditor: NodeEditor = {
    handleOperatorNameChange,
    handleAstNodeConstantChange,
    handleAddAstNodeOperand,
    // handleDeleteAstNode,
  };

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

  const { iterationValidation } = useIterationValidation({
    service: services().scenarioService,
    loadingDispatcher: pageLoadingDispatcher,
    iterationId,
  });

  return (
    <>
      <DelayedLinearProgress loading={pageLoading} />
      <Container
        sx={{
          maxWidth: "md",
        }}
      >
        {/* List of iterations */}
        <Stack direction="column" spacing={2}>
          {scenario && iterationId == null && (
            <>
              <Typography variant="h5">
                {scenario.allIterations.length} iterations
              </Typography>
              <Box
                sx={{
                  bgcolor: "background.paper",
                }}
              >
                <List>
                  {scenario.allIterations.map((iteration) => (
                    <ListItem key={iteration.iterationId} disablePadding>
                      <ListItemButton
                        onClick={() =>
                          handleIterationClick(iteration.iterationId)
                        }
                      >
                        <ListItemAvatar>
                          <Avatar>
                            <MovieIcon />
                          </Avatar>
                        </ListItemAvatar>
                        <ListItemText
                          primary={
                            iteration.version === null
                              ? "Draft"
                              : `Live (version ${iteration.version})`
                          }
                          secondary={`Updated: ${detailedDateFormat.format(
                            iteration.updatedAt
                          )}`}
                        ></ListItemText>
                      </ListItemButton>
                    </ListItem>
                  ))}
                </List>
              </Box>
            </>
          )}

          {/* Iteration */}
          {iteration && (
            <>
              <Typography variant="h5">
                {iteration.version ? (
                  <>Live Iteration (version {iteration.version})</>
                ) : (
                  <>Draft Iteration</>
                )}
              </Typography>
              <TriggerCondition
                onEditTrigger={iterationEditable ? handleEditTrigger : null}
                triggerCondition={iteration.triggerCondition}
                validation={
                  iterationValidation
                    ? iterationValidation.triggerEvaluation
                    : null
                }
              />
              {iteration.rules.map((rule) => (
                <RuleComponent
                  key={rule.ruleId}
                  onEditRule={
                    iterationEditable ? () => handleEditRule(rule.ruleId) : null
                  }
                  rule={rule}
                  validation={
                    iterationValidation
                      ? iterationValidation.rulesEvaluations[rule.ruleId]
                      : null
                  }
                />
              ))}
            </>
          )}

          {/* 
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
  validation,
}: {
  onEditTrigger: (() => void) | null;
  triggerCondition: AstNode | null;
  validation: AstNodeEvaluation | null;
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
          <AstNodeComponent
            node={triggerCondition}
            evaluation={validation}
            displaySuccess={false}
          />
        </>
      )}
    </>
  );
}

function RuleComponent({
  rule,
  onEditRule,
  validation,
}: {
  rule: Rule;
  onEditRule: (() => void) | null;
  validation: AstNodeEvaluation | null;
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
          <AstNodeComponent
            node={rule.formulaAstExpression}
            evaluation={validation}
            displaySuccess={false}
          />
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

// "fr-FR"
const detailedDateFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: "full",
});
