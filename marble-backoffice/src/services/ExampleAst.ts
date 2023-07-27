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

export const demoRules: ExampleRule[] = [
  {
    name: "Medium amount",
    description: "Amount is between 10k and 100k, hence medium risk",
    scoreModifier: 10,
    formula: adaptLitteralAstNode({
      name: "And",
      children: [
        {
          name: ">",
          children: [
            { name: "Payload", children: [{ constant: "amount" }] },
            { constant: 10000 },
          ],
        },
        {
          name: "<",
          children: [
            { name: "Payload", children: [{ constant: "amount" }] },
            { constant: 100000 },
          ],
        },
      ],
    }),
  },
  {
    name: "High amount",
    description: "Amount is above 100k, hence high risk",
    scoreModifier: 20,
    formula: adaptLitteralAstNode({
      name: ">",
      children: [
        { name: "Payload", children: [{ constant: "amount" }] },
        { constant: 100000 - 1 },
      ],
    }),
  },
  {
    name: "Medium risk country",
    description: "Country is in the list of medium risk (european) countries",
    scoreModifier: 10,
    formula: adaptLitteralAstNode({
      name: "IsInList",
      children: [
        { name: "Payload", children: [{ constant: "bic_country" }] },
        { constant: ["HU", "IT", "PO", "IR"] },
      ],
    }),
  },

  {
    name: "High risk country",
    description: "Country is in the list of high risk (european) countries",
    scoreModifier: 20,
    formula: adaptLitteralAstNode({
      name: "IsInList",
      children: [
        { name: "Payload", children: [{ constant: "bic_country" }] },
        { constant: ["RO", "RU", "LT"] },
      ],
    }),
  },
  {
    name: "Low risk country",
    description: "Country is domestic (France)",
    scoreModifier: -10,
    formula: adaptLitteralAstNode({
      name: "=",
      children: [
        { name: "Payload", children: [{ constant: "bic_country" }] },
        { constant: "FR" },
      ],
    }),
  },
  {
    name: "Frozen account",
    description: "The account is frozen",
    scoreModifier: 100,
    formula: adaptLitteralAstNode({
      name: "DatabaseAccess",
      named_children: {
        tableName: { constant: "transactions" },
        fieldName: { constant: "is_frozen" },
        path: { constant: ["account"] },
      },
    }),
  },
];

export const exampleTriggerCondition = NewAstNode({
  name: "And",
  children: [
    NewAstNode({
      name: "=",
      children: [
        NewAstNode({
          name: "Payload",
          children: [NewAstNode({ constant: "direction" })],
        }),
        NewAstNode({ constant: "payout" }),
      ],
    }),
    NewAstNode({
      name: "=",
      children: [
        NewAstNode({
          name: "Payload",
          children: [NewAstNode({ constant: "status" })],
        }),
        NewAstNode({ constant: "pending" }),
      ],
    }),
  ],
});
