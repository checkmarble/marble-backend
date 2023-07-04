import services from "@/injectServices";
import Container from "@mui/material/Container";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { AstNode, NoConstant } from "@/models";
import Paper from "@mui/material/Paper";
import Alert from "@mui/material/Alert";
import TextField from "@mui/material/TextField";
import AddIcon from "@mui/icons-material/Add";
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

export default function ScenarioDetailsPage() {
  // const { scenarioId } = useParams();
  // const navigate = useNavigate();

  //   const { scenarios } = useScenarios(
  //     services().organizationService,
  //     pageLoadingDispatcher,
  //     organizationId
  //   );

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { editor, expressionAstNode, validate, validationErrors, run } =
    useAstExpressionBuilder(
      services().astExpressionService,
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
          <Typography variant="h5">Expression Editor</Typography>

          {/* <AstEditor
            editor={nodeEditor}
            node={editor.expressionViewModel.rootNode}
          /> */}

<Button onClick={handleValidateScenario}>Validate</Button>
<Button onClick={handleRunScenario}>Run</Button>
          {validationErrors.map((error, i) => (
            <Alert key={i} severity="error">
              {error}
            </Alert>
          ))}

          <Typography variant="h5">Result</Typography>
          <AstNode node={expressionAstNode} />
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

function AstNode(props: { node: AstNode }) {
  const node = props.node;
  return (
    <>
      <Paper
        sx={{
          margin: 2,
          padding: 1,
          border: 1,
        }}
      >
        {node.name && <Typography variant="subtitle1">{node.name}</Typography>}
        {node.constant !== NoConstant && (
          <Typography>
            Constant: <code>{JSON.stringify(node.constant)}</code>
          </Typography>
        )}
        {!node.name && node.constant === NoConstant && (
          <Typography>
            ⚠️ Invalid Node: Not a constant, not a function
          </Typography>
        )}
        {node.children.map((child, i) => (
          <AstNode key={i} node={child} />
        ))}
        <div>
          {Object.entries(node.namedChildren).map(([name, child], i) => (
            <div key={i}>
              <Typography variant="subtitle2">{name}</Typography>{" "}
              <AstNode node={child} />
            </div>
          ))}
        </div>
      </Paper>
    </>
  );
}
