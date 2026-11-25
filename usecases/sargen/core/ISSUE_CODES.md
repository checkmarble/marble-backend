# Issue codes

An issue is a problem found while validating a declaration document. Each issue has:

- `path`: the concrete path of the field or group concerned (may be empty for rules without a scope);
- `code`: a stable identifier; the front end switches on it;
- `severity`: `blocking` (the declaration cannot be exported; a draft is still saved) or `warning`. The validator only reports: `WriteXML` does not validate, so the caller refuses the export while a blocking issue remains;
- `params`: string-to-string map with the values needed to build the sentence; **numbers are decimal strings**; absent when the code has no params;
- `message`: an English text for logs and debugging only. Do not display it;
- `rule_id`: set for `rule:<id>` issues.

The front end translates from `code` + `params`. Rule issues (`rule:<id>`) are translated by rule id; their `message` is the rule description from the format spec (English).

## Core codes

| code | severity | when | params |
|---|---|---|---|
| `unknown_path` | blocking | the path is malformed or not in the format | none |
| `unknown_item` | blocking | a value sits under an item (keyed segment) that does not exist | none |
| `export_id_value` | blocking | a value is given for an identifier generated at export | none |
| `not_a_field` | blocking | the path designates a group, not a field | none |
| `bad_char` | blocking | character not allowed in XML (control char, invalid UTF-8) | none |
| `bad_boolean` | blocking | not `true` / `false` | none |
| `bad_date` | blocking | not a `YYYY-MM-DD` date | `expected` = `YYYY-MM-DD` |
| `bad_integer` | blocking | not an integer | none |
| `bad_decimal` | blocking | not a decimal number (no params), or too many fraction digits | none, or `max_fraction_digits` |
| `max_length` | blocking | too long: strings and codes (characters), integers and decimals (significant digits) | `max`, `unit` (`characters` or `digits`); `present` for characters |
| `min_length` | blocking | too short | `min`, `present`, `unit` = `characters` |
| `pattern` | blocking | value does not match the format pattern | none |
| `bad_pattern` | blocking | the format's own pattern is unreadable (spec bug) | `pattern` |
| `unknown_code` | blocking | value not in the code list (or code list unknown) | `code_list`, `value` |
| `free_text_code` | warning | a `code_or_text` field holds a value that is not a code of its list: accepted as free text, to check | `code_list`, `value` |
| `code_list_unavailable` | warning | the code list is not published, value not checked | `code_list` |
| `bad_ref` | blocking | reference to an item of a type that is not allowed | `value` |
| `dangling_ref` | blocking | reference to an item that does not exist in the document | `value` |
| `required` | blocking, or warning for working (non-exported) fields | mandatory field without value | none |
| `too_few_items` | blocking, or warning for working nodes | fewer items than the minimum | `min`, `present` |
| `too_many_items` | blocking | more items than the maximum | `max`, `present` |
| `unknown_rule` | blocking | a rule of the spec has an unknown kind or condition operator (spec bug) | `rule_id` and `kind` or `op` |
| `rule:<id>` | the rule's severity; a missing working field (`required_if`, or an empty `exactly_one` whose alternatives are all working fields) is only a warning, a conflict (`forbidden_if`, too many alternatives) keeps the rule's severity | a generic rule of the spec is violated | none |

## TRACFIN codes

| code | severity | when | params |
|---|---|---|---|
| `montant_hors_euro` | blocking | a working transaction has a currency other than EUR | `currency` |
| `rule:cadre_legal_principal` | blocking | the legal framework has none of CL01, CL02, CL03 | `codes` = `CL01,CL02,CL03` |
| `rule:personnes_au_moins_une` | blocking | the persons section holds no person | none |
| `transaction_non_comptee` | blocking | a working transaction is left out of the operations: its currency or amount is missing | `missing` = `currency` or `amount` |
| `too_many_items` on `operations.operation` | blocking | more operation lines than the 30 of the XSD; the message says to group transactions by changing their line attributes | `max`, `present` |
| `rule:references_financieres_max_20` | blocking | more than 20 financial references on one person | `max`, `present` |
| `rule:dirigeant_beneficiaire_distinct` | blocking | a director declared as not the beneficial owner is referenced as the beneficial owner | `value` (the referenced person path) |
| `rule:lien_autre_personne_distincte` | blocking | a link to an already declared person targets the person holding it | `value` (the person path) |
| `rule:periode_operations_ordonnee` | blocking | the operations period ends before it starts | `start`, `end` (YYYY-MM-DD) |
| `rule:sous_categorie_relation_autre` | warning | the "other" detail is filled but the sub-category is not an "other" code | `value` (the chosen sub-category, possibly empty) |

The other `rule:<id>` issues of TRACFIN come from `constraints.json` (generic rules) and carry no params.

## Write codes (ApplyFills)

A write (a generation fill, a manual edit, an option chosen by the analyst) is checked by `ApplyFills` before the value is stored. A refused fill carries one of the value codes above (from `ValidateValue`) or one of these, all blocking:

| code | when | params |
|---|---|---|
| `unknown_path` | empty, malformed or unknown path | none |
| `not_found` | marking as reviewed a path that has no value | none |
| `empty_new_item` | clearing a value of an item that does not exist yet | none |
| `unknown_item` | the path goes through an item that does not exist | `item` (the keyed segment) |
| `bad_provisional_key` | a provisional key `[+x]` on a node that is not repeatable | none |
| `too_many_items` | creating an item would exceed the maximum | `max` |
| `out_of_scope` | path outside the part assigned to the generation task | none |
| `missing_origin` | the write has no origin | none |
| `unknown_origin` | the origin is not one of llm, org_setting, computed, manual | `origin` |
| `bad_confidence` | the confidence is not low, medium or high | `confidence` |
| `bad_ref` | reference to a provisional item not created in the same write | `value` |
| `dangling_ref` | reference to an item that does not exist | `value` |
| `not_writable_by_llm` | the LLM writes a field it may not fill (setting, computed, analyst text) | none |
| `origin_not_allowed` | an organization setting written by an origin other than org_setting or manual, or the org_setting origin on a field that is not a setting | `origin` |
| `missing_source` | a copied value cites no source | none |
| `missing_justification` | a judgment has no justification or confidence | none |
| `source_out_of_scope` | the cited source is outside the objects allowed for the field | none |

## Export errors

`WriteXML` refuses a document with an `*core.ExportError` (`errors.As`): `Code`, `Path`, `Params`, English `Message`. Nothing is written. A failure of the underlying writer is a plain wrapped error ("cannot write XML").

| code | when | params |
|---|---|---|
| `missing_spec` | no format description | none |
| `bad_date` | a stored date is not `YYYY-MM-DD` | `value` |
| `dangling_ref` | a reference targets an item that does not exist | `value` |
