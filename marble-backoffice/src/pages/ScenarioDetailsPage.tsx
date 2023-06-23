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
import {
  useAstExpressionBuilder,
  type NodeViewModel,
} from "@/services/AstExpressionService";

export default function ScenarioDetailsPage() {
  // const { scenarioId } = useParams();
  // const navigate = useNavigate();

  //   const { scenarios } = useScenarios(
  //     services().organizationService,
  //     pageLoadingDispatcher,
  //     organizationId
  //   );

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { expressionViewModel, expressionAstNode, validate, validationErrors } =
    useAstExpressionBuilder(
      services().astExpressionService,
      pageLoadingDispatcher
    );

  const handleValidateScenario = async () => {
    validate();
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

          <AstEditor node={expressionViewModel.rootNode} />

          <Button onClick={handleValidateScenario}>Validate</Button>

          <Typography variant="h5">Result</Typography>
          <AstNode node={expressionAstNode} />
          {validationErrors.map((error, i) => (
            <Alert key={i} severity="error">
              {error}
            </Alert>
          ))}
        </Stack>
      </Container>
    </>
  );
}

function AstEditor(props: { node: NodeViewModel }) {
  const node = props.node;


  return (
    <Paper
      sx={{
        margin: 2,
        padding: 1,
        border: 1,
      }}
    >
      <TextField
        sx={{ mr: 2 }}
        autoFocus
        margin="dense"
        label="Function"
        variant="standard"
        value={node.name}
        // onChange={handleEmailChange}
      />
      <TextField
        sx={{ mb: 1 }}
        autoFocus
        margin="dense"
        label="Constant"
        variant="standard"
        value={node.constant}
        // onChange={handleEmailChange}
      />

      {node.children.map((child, i) => (
        <AstEditor key={i} node={child} />
      ))}
      {Object.entries(node.namedChildren).map(([name, child], i) => (
        <>
          {name} <AstEditor key={i} node={child} />
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
        {node.children.map((child, i) => (
          <AstNode key={i} node={child} />
        ))}
        {Object.entries(node.namedChildren).map(([name, child], i) => (
          <>
            {name} <AstNode key={i} node={child} />
          </>
        ))}
      </Paper>
    </>
  );
}
