import Typography from "@mui/material/Typography";
import { type AstNode, type AstNodeEvaluation, NoConstant } from "@/models";
import { AstConstantComponent } from "./AstConstantComponent";
import Paper from "@mui/material/Paper";
import Alert from "@mui/material/Alert";

export function AstNodeComponent({
  node,
  evaluation,
}: {
  node: AstNode;
  evaluation?: AstNodeEvaluation;
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
        {evaluation &&
          evaluation?.returnValue !== NoConstant &&
          node.constant === NoConstant && (
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
          <AstNodeComponent
            key={i}
            node={child}
            evaluation={evaluation ? evaluation.children[i] : undefined}
          />
        ))}
        <div>
          {Object.entries(node.namedChildren).map(([name, child], i) => (
            <div key={i}>
              <Typography variant="subtitle2">{name}</Typography>{" "}
              <AstNodeComponent
                node={child}
                evaluation={
                  evaluation ? evaluation.namedChildren[name] : undefined
                }
              />
            </div>
          ))}
        </div>
      </Paper>
    </>
  );
}
