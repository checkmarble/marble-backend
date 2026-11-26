# sargen playground

A local page to try the write path of a SAR declaration document, `core.ApplyFills`, as one of its writers. Pick a role, a field and a value, and see what happens: the verdict of each write (applied, rejected with its code, conflict with the protected value), the document with what changed, and the validation issues.

It runs the real `core` and the real TRACFIN 1.4.0 spec. No database, no LLM, no network.

## Run it

Its files build only with the `sargen_playground` tag: the server build, `go build ./...`, `go test ./...` and the linters of the repository never see it. It uses the repository's own `go.mod`, so it has no dependency of its own to maintain. Run it from its own folder:

```bash
cd usecases/sargen/playground
go run -tags sargen_playground .
```

Then open http://localhost:8099 (`-addr` to change it). Its tests, from the same folder:

```bash
go test -tags sargen_playground .
```

## Starting documents

- **Déclaration générée** (default, `starts/generated.json`): a valid declaration with the provenance a generation gives: organization settings, computed fields with their computation, the LLM's values with their sources (and a justification and confidence for judgments), the analysis typed by the analyst. Nothing is reviewed yet. The scenarios start from it.
- **Document vide**.

## Roles

Each role writes as the stack does (later PRs):

| Role | Origin | Overwrite policy | Extra inputs |
|---|---|---|---|
| LLM | `llm` | `non_manual` | sources, justification, confidence, task scope (`AllowedPrefixes`) |
| Analyst | `manual` | `all` | none |
| Computation | `computed` | `non_manual` | the computation inputs |
| Settings | `org_setting` | `non_manual` | none |

"Marquer relu" calls `core.MarkReviewed`. "Ajouter au lot" groups writes into one `ApplyFills` call, which shows what depends on the order within a call (provisional items `[+a]`, references to them).

The field picker lists the spec's fields with their nature; for each list segment it offers the existing items or a new provisional key, and the value input follows the field type (code list, date, boolean, reference). "Chemin libre" and "valeur libre" send what you type as is, to see every refusal.

## Scenarios

`scenarios/*.json` are ready-made stories, one per rule of the PR description. Each step gives the role, the fills and the expected outcome of each fill (`applied`, `noop`, `refreshed`, `conflict`, `rejected:<code>`, or `reviewed` for a review step). The page replays them; `TestScenarios` replays them too and fails when a rule of `ApplyFills` changes, so the scenarios stay true. To add one, add a file: no Go code to change.
