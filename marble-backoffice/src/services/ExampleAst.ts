import { type AstNode, NewAstNode } from "@/models";
import { adaptLitteralAstNode } from "@/models/AstExpressionDto";

// export const testAst = NewAstNode({
//   name: ">",
//   children: [
//     NewAstNode({ constant: 15 }),
//     NewAstNode({
//         name: "*",
//         children: [
//             NewAstNode({ constant: 4 }),
//             NewAstNode({ constant: 3 }),
//             // NewAstNode({ constant: "toto" }),
//         ],
//      }),
//   ],
// });

export interface ExampleRule {
  name: string;
  description: string;
  formula: AstNode;
  // displayOrder: number;
  scoreModifier: number;
}

export const exampleRuleInList: ExampleRule = {
  name: "testRuleInList",
  description: "testRuleInList",
  scoreModifier: 10,
  formula: NewAstNode({
    name: "IsInList",
    children: [
      NewAstNode({
        name: "DatabaseAccess",
        namedChildren: {
          tableName: NewAstNode({ constant: "transactions" }),
          fieldName: NewAstNode({ constant: "name" }),
          path: NewAstNode({ constant: ["account"] }),
        },
      }),
      NewAstNode({
        name: "CustomListAccess",
        namedChildren: {
          customListId: NewAstNode({
            constant: "d6643d7e-c973-4899-a9a8-805f868ef90a",
          }),
        },
      }),
    ],
  }),
};

function adaptLitteralAnds(ands: unknown[]): AstNode {
  return adaptLitteralAstNode({
    name: "Or",
    children: [
      {
        name: "And",
        children: ands,
      },
    ],
  });
}

export const demoRules: ExampleRule[] = [
  {
    name: "Medium amount",
    description: "Amount is between 10k and 100k, hence medium risk",
    scoreModifier: 10,
    formula: adaptLitteralAnds([
      {
        name: ">=",
        children: [
          { name: "Payload", children: [{ constant: "value" }] },
          { constant: 10000 },
        ],
      },
      {
        name: "<",
        children: [
          { name: "Payload", children: [{ constant: "value" }] },
          { constant: 100000 },
        ],
      },
    ]),
  },
];

export const exampleTriggerCondition = adaptLitteralAstNode({
  name: "And",
  children: [
    {
      name: "=",
      children: [
        { name: "Payload", children: [{ constant: "direction" }] },
        { constant: "PAYOUT" },
      ],
    },
    {
      name: "=",
      children: [
        { name: "Payload", children: [{ constant: "status" }] },
        { constant: "PENDING" },
      ],
    },
  ],
});
