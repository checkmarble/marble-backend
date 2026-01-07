## 🎯 Recursive Arithmetic & Boolean Formula Builder

This exercise requires you to implement a reusable **Formula Expression** component that allows users to create nested formulas with both arithmetic and boolean operators. The primary challenge is designing a type-consistent state structure where the entire formula tree is validated at every level: input types must match operator requirements, intermediate types must be consistent, and the root must always evaluate to a boolean.

---

## Part 1: Setting and Hardcoded Data

For this exercise, all necessary data must be **hard-coded** directly into the component or a constants file. In real conditions, some of them at least may be provided by an API, we ignore this complexity for the exercise.

### 1. Hardcoded Variables

| Variable Name                  | Type      |
| :----------------------------- | :-------- |
| `transaction.amount`           | `int`     |
| `transaction.account_name`     | `string`  |
| `transaction.account.balance`  | `int`     |
| `transaction.sender.last_name` | `string`  |
| `transaction.has_3DS`          | `boolean` |

### 2. Hardcoded Operators

| Operator ID      | Label            | Input Types       | Output Type | Category   |
| :--------------- | :--------------- | :---------------- | :---------- | :--------- |
| `gt_eq`          | `>=`             | `int`             | `boolean`   | Comparison |
| `eq`             | `=`              | `int` or `string` | `boolean`   | Comparison |
| `neq`            | `!=`             | `int` or `string` | `boolean`   | Comparison |
| `is_close_match` | `is close match` | `string`          | `boolean`   | Comparison |
| `add`            | `+`              | `int`             | `int`       | Arithmetic |
| `subtract`       | `-`              | `int`             | `int`       | Arithmetic |
| `multiply`       | `*`              | `int`             | `int`       | Arithmetic |
| `divide`         | `/`              | `int`             | `int`       | Arithmetic |
| `AND`            | `AND`            | `boolean`         | `boolean`   | Logical    |
| `OR`             | `OR`             | `boolean`         | `boolean`   | Logical    |

---

## Part 2: The Core Rule Component (Target Time: ~1 hour)

**Objective:** Implement the base `FormulaExpression` component: `[Left Input] [Operator] [Right Input]`.

In Part 2, all operators are treated uniformly as simple binary operators. AND/OR are just like the comparison operators—they simply accept two inputs and return a boolean. No special nesting logic yet.

### Requirements

1.  **UI Implementation:** Render the three fields. Values must be populated from the hardcoded lists above.
2.  **Dynamic Filtering:** When the user selects a variable in the **Left Input** field, the **Operator** dropdown must **immediately filter** to show only compatible operators based on the variable's type. The **Left Input** may take a constant value or a variable.
3.  **Dynamic Right Input & Validation:**
    - The **Right Input** field must allow _either_ selecting a compatible variable _or_ entering a constant value.
    - The Right Input type must match the operator's input requirements.
4.  **Prioritized Validation (Base Level):** Implement the following prioritized validation rules:
    - **Highest Priority:** Fields are not filled.
    - **Second Priority:** Incompatible types between Left and Right inputs for the selected operator.

---

## Part 3: The nested expressions (Target Time: ~2 hour)

**Objective:** Refactor the component to support nesting with arithmetic and boolean operators, ensuring **full type consistency at every level**.

### The Core Challenge: Recursive Operators with Type Validation

Part 3 introduces nesting where operators can accept nested sub-expressions as inputs:

- **Arithmetic Operators** (e.g., `+`, `-`): Accept numeric inputs/expressions, return `int`, can be nested.
- **Comparison Operators** (e.g., `>=`, `=`): Accept numeric/string inputs/expressions, return `boolean`, can be nested.
- **Logical Operators** (`AND`, `OR`): Accept boolean inputs/expressions, return `boolean`, can be nested.

### Requirements

1.  **State Architecture:** Design a structure to hold the entire formula tree. Each node can be either:
    - An **atomic value** (variable or constant)
    - An **operator node** with left and right children (which can themselves be atomic or operator nodes)
2.  **Type Validation at Every Level:**

    - Each operator node validates that its left and right inputs have the correct input type.
    - Each operator's output type is tracked through the tree.
    - The root formula must evaluate to `boolean` type.
    - Type validation errors must clearly indicate which level failed and why.

3.  **Conditional Recursive Rendering:**

    - When the user selects an operator with output type `int` (e.g., `+`), the inputs must accept either:
      - Variables/constants of type `int`
      - **Nested formulas that output `int`** (e.g., `5 + 3` or `transaction.amount - 10`)
    - When the user selects an operator with output type `boolean` (e.g., `>=`), inputs must accept:
      - Variables/constants compatible with the operator's input type
      - **Nested formulas that output the correct type**
    - The UI must visually enforce these rules (previous inputs disappear/transform when operator changes).

4.  **State Management:**

    - Implement the core state logic to ensure nested formula changes propagate correctly to the root.
    - Implement **Delete/Reset** functionality for nested expressions.
    - Validation must cascade: if a nested formula becomes invalid, the parent formula is also marked invalid.

5.  **Global Validation Constraint:**
    - The entire formula tree is only valid if:
      - All intermediate nodes have type-consistent inputs and outputs
      - The root node outputs `boolean`
      - No node has incomplete fields
