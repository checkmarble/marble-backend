# Automatic index creation

## Context

Marble is a decision engine for fraud and AML that lets users define their custom data model and ingest their own data to create decisions on.
This provides great flexibility for banks that want to score different payment rails, user events, possibly in several different countries. In contrast, traditional decision engines often constrain users to shoehorn their data into a unified and very opinionated data model - or provide flexibility at high cost of professional services.

On the other hand, we still wanted our decision engine to let customers use arbitrary aggregates in their fraud detection rules, without requiring extensive pre-processing before data is even ingested: we believe that ingesting data into the decision engine should be a one-time setup by the technical teams in charge, and that iterating on the detection rules, using new metrics, should be only a click away for the compliance and operational teams that operate the tool: no time-consuming back and forth between tech and operations.

## The technical challenge

Exposing this abstraction - ingest your data, and Marble will take care of computing your aggregates for you - while guaranteeing a level of quality of service means that our real-time engine needs to be smart, and handle data preparation under the hood in an opinionated way.

This article presents the first version we implemented of automatic indexing of customer-ingested data. We have made some changes since then, but the main ideas remain the same.

## The underlying infrastructure

The simplest Marble version uses a PostgreSQL database to store most of its data. The custom tables that hold data ingested by customers are held in a dedicated schema per customer - that may optionally be a dedicated database instance per customer.

The Marble tool is aware of the nature of the columns that are ingested: type (number, text, timestamp, boolean), cardinality (low-cardinality AKA "enum" or high-cardinality for text values), required or optional...

For the sake of simplicity, assume we have an `org_data` schema, containing 3 tables: `transactions`, `accounts`, `users`. The tables are related through informal foreign keys (a mapping between fields of different tables, without actual Postgres foreign keys). Consider further that the `transactions` table is the one that contains high volume data, with millions of rows added per month, versus thousands for `accounts` and `users`.

## Nature of aggregates

The idea behind Marble is that customers can use the data they loaded into the tool to compute all the metrics that are useful for their decision-making. Here are some examples:

- in a customer onboarding context, number of existing customers sharing the same name, address, or similar
- in a transaction monitoring context:
  - number of, or average value of payments made to a given beneficiary in the past
  - total value of payments received from abroad during the month so far
  - number of payments that had an incident, and that have a label (free text) similar to the one being scored

Critically, the filters received on the aggregates may be constant values defined in the rules, or may be dynamical values received in the payload of a transaction to score, or from a constantly evolving allowlist of account identifiers, and so on.
This means that we cannot easily precompute them - though it is possible to do so by placing stronger restrictions on the nature of the aggregate or the way data is ingested.

Marble aggregates using the following functions: `COUNT`, `COUNT DISTINCT`, `SUM`, `MAX`, `MIN`, `AVG`.
It further allows to filter data with the following filter types:

- equality or inequality (`!=`, `>`, ...) conditions for all types
- presence or absence of the value in a list, for text values
- value begins or ends with a given text, for text values
- value is absent ("null") or empty, for all types
- text is similar to a given text, for text values (using string similarity functions, not detailed here)

The combination of the aggregate definition, and the input values for filters that are received at runtime from the payload or other input, are translated under the hood into a SQL query, that may look like this:

```sql
SELECT SUM(amount)
FROM org_data.transactions
WHERE account_id=$1
   AND transaction_at>$2
   AND transaction_at<$3
   AND payment_method=$4
   AND counterparty_country=ANY($5)
```

where `$1` to `$5` are input values computed at runtime.

Scenarios and rules in Marble are versioned. When a customer activates a new version of a rule, including possibly new aggregates, Marble knows the exact set of aggregates that will be used in this version.

### A short primer on B-tree indexing

In order to compute those aggregates efficiently, Marble indexes the ingested data using B-tree secondary indexes.

This article does _not_ aim to explain how B-tree indexing in Postgres works. This paragraph only sums up the minimum context useful for the end of the article:

A B-tree index in PostgreSQL stores data in a sorted, hierarchical structure. For a multi-column index on columns `(A, B, C)`, entries are sorted first by `A`, then by `B` within each distinct value of `A`, then by `C` within each distinct `(A, B)` pair.

This sorting enables efficient index usage when queries filter on a **prefix** of the indexed columns. The key insight: PostgreSQL can efficiently use an index on `(A, B, C)` when:

1. **Equality matching on a prefix**: Queries with equality conditions on the first N columns (`A=x`, `A=x AND B=y`, or `A=x AND B=y AND C=z`) can use the index efficiently. The query planner narrows down to a specific subset of the sorted structure.

2. **Optional inequality on the final column**: After equality matches on a prefix, an inequality condition (`<`, `>`, `<=`, `>=`) on the next column can still use the index. For example, with equality on `A` and `B`, a range condition `C > z` works well because within the matching `(A, B)` pairs, entries are sorted by `C`.

However, the index becomes less useful if:

- Columns are queried out of order (e.g., filtering on `B` and `C` but not `A`)
- Multiple inequality conditions are used (e.g., `B > x AND C > y`) - only the first inequality benefits from the index
- Skipping columns in the prefix (e.g., `A=x AND C=z` without filtering on `B`)

For our aggregate queries with multiple `WHERE` conditions, this means that **column order in the index matters tremendously**. An index on `(account_id, transaction_at, payment_method)` efficiently supports equality on `account_id` plus a range on `transaction_at`, but not the reverse.

## Our heuristic for automatic data indexing

To simplify things, the first version of our automatic indexing heuristic makes the following opinionated choices:

1. all aggregates should be able to rely on index-only scans
2. no introspection is done on ingested data
3. given available information, the heuristic tries to create "as few indexes as possible", in order to minimize write performance and disk size impact

Following iterations make the rules more complex, without fundamentally chaning the message of this article.

### Index lifecycle management

Our approach balances two competing concerns: ensuring queries have the indexes they need, while avoiding index bloat that degrades write performance and consumes disk space.

The workflow operates in two phases:

1. **Index creation at publish time**: When a customer publishes a new version of their rules (which may include new aggregates), Marble analyzes the queries that will be executed and creates only the additional indexes required.

2. **Periodic cleanup**: A batch process runs regularly to remove indexes that are no longer needed by any active rule version. This can be configured to preserve indexes for recently-used versions, allowing quick rollbacks without re-indexing.

This lifecycle means indexes are always present when needed, but obsolete indexes don't accumulate indefinitely as rules evolve.

### Representing index requirements flexibly

The core challenge is that many similar queries can often share the same index. Creating one index per query would be wasteful; we want to identify opportunities to consolidate.

To enable this, we represent index requirements using a flexible structure with four components:

- **Fixed**: An ordered list of columns that must appear first in the index, in that specific order. This is crucial for queries that require a specific column ordering to satisfy their equality filters.

- **Flex**: An unordered set of columns that should be indexed but whose order is flexible. These typically correspond to equality filters where any ordering would work.

- **L** (Last): A single column for inequality conditions. Remember from the B-tree primer that only one inequality condition can efficiently use the index, and it should come after all equality conditions.

- **O** (Others): Columns to include using PostgreSQL's `INCLUDE` clause for index-only scans. These aren't part of the indexed structure but are stored in the index for retrieval without touching the table.

For example, for our earlier query with `account_id=$1 AND transaction_at>$2 AND transaction_at<$3 AND payment_method=$4`, we might generate:

- Fixed: `[]` (nothing required in a specific position)
- Flex: `{account_id, payment_method}` (both have equality conditions)
- L: `transaction_at` (has inequality conditions)
- O: `{amount}` (the aggregated column)

This represents a family of possible indexes: `(account_id, payment_method, transaction_at)` or `(payment_method, account_id, transaction_at)`, both with `amount` included.

### The algorithm: build and merge

The algorithm proceeds in two phases:

**Phase 1: Generate base requirements**

For each aggregate query, we analyze its filters to determine what index structure(s) could efficiently serve it. The construction logic depends on the filter types:

- **Equality filters** (`=`, `IN`): All columns with equality filters go into the Flex set, since any ordering of equality-matched columns works equally well
- **Inequality filters** (`<`, `>`, `≤`, `≥`): Here's where it gets interesting
- **Other filters** (`!=`, `LIKE`, similarity, etc.): These cannot efficiently use B-tree indexes for filtering, but the columns should be added to O for index-only scans

The tricky case is handling **multiple inequality conditions**. Recall that a B-tree index can efficiently support only one inequality column - the one that comes last in the index, after all equality columns.

Consider a query like:

```sql
SELECT COUNT(*)
FROM transactions
WHERE account_id = $1
  AND transaction_at > $2
  AND amount > $3
```

We have equality on `account_id` and inequalities on both `transaction_at` and `amount`. Which inequality should we favor in the index? The answer depends on data distribution:

- If `transaction_at > $2` typically filters out 90% of rows, we want `(account_id, transaction_at)`
- If `amount > $3` is more selective, we want `(account_id, amount)` instead

**Without data introspection, we don't know which is better**. Our opinionated choice: generate one index family for each inequality column. For the query above, we create two families:

- Family 1: Flex=`{account_id}`, L=`transaction_at`, O=`{amount}`
- Family 2: Flex=`{account_id}`, L=`amount`, O=`{transaction_at}`

This means a single aggregate query with N inequality conditions generates N candidate index families. This may seem wasteful, but the merge phase (Phase 2) will consolidate where possible. If other queries use the same combinations, we won't create redundant indexes.

In practice, most aggregates have at most 1-2 inequality conditions, commonly on timestamp fields for time ranges. So the explosion is limited. More advanced cases are handled with data introspection and are not discussed here.

**Summary of base construction**

To recap, for each aggregate query:

1. Identify all equality filters → place columns in Flex
2. If there are inequality filters on K different columns → create K separate index families, each with:
   - The same Flex set (equality columns)
   - One of the K inequality columns as L
   - All other columns (including other inequality columns) in O
3. If there are no inequality filters → create one family with just Flex and O populated

The key insight: we're deferring the decision about which inequality to favor. Instead, we generate all plausible options and let the merge phase determine which indexes actually need to be created based on what other queries need.

**Phase 2: Merge compatible families**

Now we iteratively reduce the set by merging compatible index families. For each new index family, we check if it can be merged with any existing family in our output list.

Two index families can merge if we can construct a single index that satisfies both. The key compatibility rules:

- If two families have the same Fixed prefix, we can potentially merge their Flex sets and reason about their L columns
- If one family's Fixed prefix is entirely contained in another's Flex set, they might be compatible
- If both families use the same L column (or one has no L), merging is more likely to succeed

For instance:

- Family A: Flex=`{account_id}`, L=`transaction_at`
- Family B: Flex=`{account_id, payment_method}`, L=`transaction_at`

These merge into: Flex=`{account_id, payment_method}`, L=`transaction_at`

The resulting index, say `(account_id, payment_method, transaction_at)`, serves both queries.

However, if the L columns differ:

- Family A: Flex=`{account_id}`, L=`transaction_at`
- Family C: Flex=`{account_id}`, L=`amount`

These cannot merge into a single efficient index, since we can only have one inequality column benefit from indexing.

The merging logic involves checking whether the columns from one family can be accommodated in the other's structure while preserving the B-tree prefix requirements. This is somewhat intricate but follows directly from the constraints we discussed in the B-tree primer.

### Practical outcomes and trade-offs

This algorithm is **not guaranteed to find the absolute minimal set of indexes** - that problem is likely NP-hard in the general case. However, it runs in polynomial time and reliably catches the most common consolidation opportunities.

In practice, we've found it strikes a good balance:

- Most aggregates that filter on the same high-cardinality column (like `account_id`) plus various combinations of other filters end up sharing indexes
- Aggregates with fundamentally different access patterns get separate indexes
- The system avoids creating redundant indexes like `(A, B, C)` and `(A, B)` when the former suffices

**The multiple inequality challenge**

However, the approach has inherent limitations due to our data-agnostic stance. Consider two queries:

- Query 1: `WHERE account_id=$1 AND transaction_at>$2 AND amount>$3`
- Query 2: `WHERE account_id=$1 AND created_at>$4 AND amount>$5`

Both have inequalities on `amount`. Following our algorithm, we'd generate index families including:

- From Query 1: Flex=`{account_id}`, L=`amount`, O=`{transaction_at}`
- From Query 2: Flex=`{account_id}`, L=`amount`, O=`{created_at}`

These can merge into a single index on `(account_id, amount)`. Great! But both queries also generated alternative families favoring their respective timestamp columns, which won't merge since `transaction_at ≠ created_at`. We might end up with three indexes where ideally we'd pick just one.

**Without data statistics**, we can't determine that the merged `amount` index is "good enough" and drop the timestamp alternatives. A more sophisticated system can:

- Monitor which indexes are actually used by the query planner
- Collect statistics on filter selectivity
- Drop underperforming indexes after observing real query patterns

Our v1 accepted these limitations. The tradeoff: it may create a few more indexes than theoretically optimal, but we guarantee good query performance without requiring manual tuning or waiting for statistics to accumulate.

This opinionated, data-agnostic approach has served us well. It allows customers to iterate freely on their detection rules, knowing that Marble will handle the indexing transparently, without requiring them to understand database internals or wait for professional services to optimize their queries.

## Conclusion

Automatic index creation sits at the intersection of user experience and database optimization. By making opinionated choices—favoring index-only scans, consolidating where possible, cleaning up proactively—we've built a system that lets fraud analysts focus on detection logic rather than query performance.

The core insight is that B-tree index semantics impose structure on which indexes are useful, and that structure can be exploited algorithmically to reduce the combinatorial explosion of possible indexing strategies. While not perfect, this heuristic approach has scaled well as our customers' data models and rule complexity have grown.
