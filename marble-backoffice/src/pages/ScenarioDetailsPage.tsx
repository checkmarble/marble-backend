import services from "@/injectServices";
import Container from "@mui/material/Container";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { type AstNode, Rule } from "@/models";
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
import { useParams } from "react-router-dom";
import { useSingleScenario } from "@/services";
import {
  AstNodeComponent,
  AstNodeTextComponent,
} from "@/components/AstNodeComponent";

export default function ScenarioDetailsPage() {
  const { scenarioId } = useParams();

  if (!scenarioId) {
    throw Error("scenarioId is required");
  }
  // const navigate = useNavigate();

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { scenario } = useSingleScenario(
    services().organizationService,
    pageLoadingDispatcher,
    scenarioId
  );

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

          {scenario?.liveIteration && (
            <>
              <TriggerCondition
                triggerCondition={scenario.liveIteration.triggerCondition}
              />
              {scenario.liveIteration.rules.map((rule) => (
                <RuleComponent key={rule.ruleId} rule={rule} />
              ))}
            </>
          )}
        </Stack>
      </Container>
    </>
  );
}

function TriggerCondition({
  triggerCondition,
}: {
  triggerCondition: AstNode | null;
}) {
  if (triggerCondition === null) {
    return <>No trigger condition</>;
  }

  return (
    <>
      <Typography variant="h6">Trigger condition</Typography>
      <AstNodeTextComponent node={triggerCondition} />
      <AstNodeComponent node={triggerCondition} />
    </>
  );
}

function RuleComponent({ rule }: { rule: Rule }) {
  return (
    <>
      <Typography variant="h6">Rule {rule.name}</Typography>
      <Typography variant="subtitle1">Rule {rule.description}</Typography>
      {rule.formulaAstExpression === null ? (
        <>No formula</>
      ) : (
        <>
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
