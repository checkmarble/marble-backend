import { AstNode } from "./AstExpression";

export interface Identifier {
  name: string;
  description: string;
  node: AstNode;
}

export interface EditorIdentifiers {
  databaseAccessors: Identifier[];
  payloadAccessors: Identifier[];
  customListAccessors: Identifier[];
}
