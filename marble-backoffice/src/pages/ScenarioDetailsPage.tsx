import services from "@/injectServices";
import Container from "@mui/material/Container";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import {
  type AstNode,
  type AstNodeEvaluation,
  ConstantOptional,
  NoConstant,
} from "@/models";
import Paper from "@mui/material/Paper";
import Alert from "@mui/material/Alert";
import TextField from "@mui/material/TextField";
import AddIcon from "@mui/icons-material/Add";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
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
import { useParams } from "react-router-dom";
import { boolean } from "yup";

export default function ScenarioDetailsPage() {
  const { scenarioId } = useParams();

  if (!scenarioId) {
    throw Error("scenarioId is required");
  }
  // const navigate = useNavigate();

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const {
    editor,
    expressionAstNode,
    validate,
    validationErrors,
    dryRunResult,
    run,
    identifiers,
  } = useAstExpressionBuilder(
    services().astExpressionService,
    scenarioId,
    pageLoadingDispatcher
  );

  const handleValidateScenario = async () => {
    validate();
  };

  const handleRunScenario = async () => {
    run();
  };

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

  return (
    <>
      <DelayedLinearProgress loading={pageLoading} />
      <Container
        sx={{
          maxWidth: "md",
        }}
      >
        <Stack direction="column" spacing={2}>
          {/* <Typography variant="h5">Expression Editor</Typography>

          <AstEditor
            editor={nodeEditor}
            node={editor.expressionViewModel.rootNode}
          /> */}

          <Button onClick={handleValidateScenario}>Validate</Button>
          {validationErrors.length > 0 && (
            <Card>
              <CardContent>
                <Typography
                  sx={{ fontSize: 14 }}
                  color="text.secondary"
                  gutterBottom
                >
                  Validation errors
                </Typography>
                {validationErrors.map((error, i) => (
                  <Alert key={i} severity="error">
                    {error}
                  </Alert>
                ))}
              </CardContent>
            </Card>
          )}

          <Button onClick={handleRunScenario}>Run (Ingestion required)</Button>
          {dryRunResult && (
            <Card>
              <CardContent>
                <Typography
                  sx={{ fontSize: 14 }}
                  color="text.secondary"
                  gutterBottom
                >
                  Dry run result:{" "}
                  <AstConstantComponent constant={dryRunResult.returnValue} />
                </Typography>

                {dryRunResult.returnValue === NoConstant && (
                  <>
                    <Typography variant="subtitle1" gutterBottom>
                      Runtime Error
                    </Typography>
                    <Alert severity="error">
                      <ReactJson src={dryRunResult} theme={"rjv-default"} />
                    </Alert>
                  </>
                )}
              </CardContent>
            </Card>
          )}

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
          <Typography variant="h5">Simple rendering of the AST</Typography>
          <AstNode node={expressionAstNode} evaluation={dryRunResult} />
        </Stack>
      </Container>
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

function AstNode({
  node,
  evaluation,
}: {
  node: AstNode;
  evaluation: AstNodeEvaluation | null;
}) {
  return (
    <>
      <Paper
        sx={{
          margin: 2,
          padding: 1,
          border: 1,
        }}
      >
        {node.name && (
          <Typography variant="subtitle1">name: {node.name}</Typography>
        )}
        {evaluation && evaluation?.returnValue !== NoConstant && node.constant === NoConstant && (
          <Alert severity="success">
            Evaluation success:{" "}
            <AstConstantComponent constant={evaluation.returnValue} />
          </Alert>
        )}
        {node.constant !== NoConstant && (
          <Typography>
            Constant: <AstConstantComponent constant={node.constant} />
          </Typography>
        )}
        {!node.name && node.constant === NoConstant && (
          <Typography>
            ⚠️ Invalid Node: Not a constant, not a function
          </Typography>
        )}
        {evaluation?.evaluationError && (
          <Alert severity="error">{evaluation.evaluationError}</Alert>
        )}
        {node.children.map((child, i) => (
          <AstNode
            key={i}
            node={child}
            evaluation={evaluation === null ? null : evaluation.children[i]}
          />
        ))}
        <div>
          {Object.entries(node.namedChildren).map(([name, child], i) => (
            <div key={i}>
              <Typography variant="subtitle2">{name}</Typography>{" "}
              <AstNode
                node={child}
                evaluation={
                  evaluation === null ? null : evaluation.namedChildren[name]
                }
              />
            </div>
          ))}
        </div>
      </Paper>
    </>
  );
}

function AstConstantComponent({ constant }: { constant: ConstantOptional }) {
  if (constant === NoConstant) {
    return <>!No Constant!</>;
  }
  if (constant === null) {
    return <>NULL</>;
  }
  return <code>{JSON.stringify(constant)}</code>;
}
