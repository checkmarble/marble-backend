import { NewAstNode } from "@/models";

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

export const testAst = NewAstNode({
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
});
