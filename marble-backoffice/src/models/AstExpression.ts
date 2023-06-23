export interface AstNode {
  name: string;
  constant: ConstantOptional;
  children: AstNode[];
  namedChildren: Record<string, AstNode>;
}

export type ConstantType = number | string | boolean | null | Array<ConstantType> | { [key: string]: ConstantType }

// NoConstant could be replaced by undefined
// slightly overengineer contant value, I hope it is still readable.
export const NoConstant : unique symbol = Symbol()
export type ConstantOptional = ConstantType | typeof NoConstant

// helper
export function NewAstNode(p: {
  name?: string;
  constant?: ConstantType;
  children?: AstNode[];
  namedChildren?: Record<string, AstNode>;
}): AstNode {
  return {
    name: p.name || "",
    constant: p.constant === undefined ? NoConstant : p.constant,
    children: p.children || [],
    namedChildren: p.namedChildren || {},
  };
}
