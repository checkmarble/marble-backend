import Typography from "@mui/material/Typography";
import { type AstNode, type AstNodeEvaluation, NoConstant } from "@/models";
import { AstConstantComponent } from "./AstConstantComponent";
import Paper from "@mui/material/Paper";
import Alert from "@mui/material/Alert";

function stringifyAst(node: AstNode): string {
  if (node.constant !== NoConstant) {
    return JSON.stringify(node.constant);
  }

  const children = node.children.map(stringifyAst);

  if (
    node.name.length <= 3 &&
    node.children.length == 2 &&
    node.namedChildren.size == 0
  ) {
    return `( ${children[0]} ${node.name} ${children[1]} )`;
  }

  if (node.namedChildren.size > 0) {
    const namedChildren: string[] = [];

    node.namedChildren.forEach((child, name) => {
      namedChildren.push(`${name}: ${stringifyAst(child)}`);
    });

    children.push(`{ ${namedChildren.join(", ")} }`);
  }

  return `${node.name}(${children.join(", ")})`;
}

export function AstNodeTextComponent({ node }: { node: AstNode }) {
  return <code>{stringifyAst(node)}</code>;
}

export function AstNodeComponent({
  node,
  evaluation,
  displaySuccess,
}: {
  node: AstNode;
  evaluation?: AstNodeEvaluation | null;
  displaySuccess: boolean;
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
          displaySuccess &&
          evaluation.returnValue !== NoConstant &&
          node.constant === NoConstant && (
            <Alert severity="success">
              Evaluation success:{" "}
              <AstConstantComponent constant={evaluation.returnValue} />
            </Alert>
          )}
        {node.constant !== NoConstant && (
          <Typography>
            Constant:{" "}
            <AstConstantComponent
              constant={node.constant}
            />
          </Typography>
        )}
        {!node.name && node.constant === NoConstant && (
          <Typography>
            ⚠️ Invalid Node: Not a constant, not a function
          </Typography>
        )}
        {evaluation?.errors &&
          evaluation.errors.map((e, i) => (
            <Alert key={i} severity="error">
              {e.error} : {e.message}
            </Alert>
          ))}
        {node.children.map((child, i) => (
          <AstNodeComponent
            key={i}
            node={child}
            evaluation={evaluation?.children[i]}
            displaySuccess={displaySuccess}
          />
        ))}
        <div>
          {Object.entries(node.namedChildren).map(([name, child], i) => (
            <div key={i}>
              <Typography variant="subtitle2">{name}</Typography>{" "}
              <AstNodeComponent
                node={child}
                evaluation={evaluation?.namedChildren[name]}
                displaySuccess={displaySuccess}
              />
            </div>
          ))}
        </div>
      </Paper>
    </>
  );
}
