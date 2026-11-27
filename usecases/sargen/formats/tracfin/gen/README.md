# TRACFIN format generator

`generate.py` produces `versions/<version>/spec.gen.json`, the description of the TRACFIN format (ERMES, XSD "financier", unit import) consumed by the validator, the XML writer, the agent and the front end. Nothing else describes the format: the form rules come from there, the hand-written business rules are in the Go code of this package.

Python 3, standard library only. The inputs are untrusted data: they are read, never executed. Always run with `python3 -I`.

## Storing a version

A format version = a `versions/<version>/` folder:

- `inputs/`: pinned XSD and CSV referentials (`inputs/financier-vX.Y.Z.xsd`, `inputs/referentiels/*.csv`);
- `manifest.json`: `format` (`tracfin:FR:<version>`), `status` (`active`, `deprecated`, `retired`) and, for each entry, `path`, `sha256`, `encoding`, `source_url`, `retrieved_on`;
- `constraints.json`: hand-written decisions (see below), each entry with a `source` field;
- `spec.gen.json`: committed result.

Old versions stay in place to open, validate and export the drafts pinned to them. The Go code (hand-written rules, steps, computations) is shared; if it must diverge, it tests the version.

## Inputs

1.4.0: XSD "financier" v1.4.0 (2026/01/21) and 22 referentials, retrieved on 2026-10-21 from <https://tracfin.finances.gouv.fr/Support/documentation-technique> (profession "Banque, établissement de crédit"). The CSVs are `windows-1252`, `;`-separated, with the `code;label` header: the script decodes them explicitly. No certification of compatibility with a real ERMES.

## Updating or adding a version

1. Create `versions/<new version>/inputs/`, copy the XSD and CSVs into it as they are (bytes unchanged).
2. Write `manifest.json` with the digests (`shasum -a 256`), the encoding of each file and the retrieval date.
3. Copy `constraints.json` from the previous version and fix it: the generator fails on any simple type missing from `type_map`, any `Ref:` reference without a binding, any template not found, any stale entry.
4. Regenerate (below) and review the diff of `spec.gen.json`.
5. The `TestGeneratedSpecIsUpToDate` test regenerates every version and compares byte for byte: a stale `spec.gen.json` fails the CI.

## Regenerating

```bash
cd usecases/sargen/formats/tracfin/gen
python3 -I generate.py --version 1.4.0 --out ../versions/1.4.0/spec.gen.json
```

Or, from the repository root: `mise run tracfin:gen`. `mise install` also provides Python and `xmllint` (needed by the XSD validation tests).

The script resolves `versions/<version>/` from its own location. Deterministic output: sorted keys, `indent=1`, `\n` line endings, no timestamp or absolute path. It fails (exit code 1, message in English) on: digest mismatch, input file missing from the manifest, simple type missing from `type_map`, list reference without a binding, XSD regex construct that Go cannot read (`\i`, `\c`, `\p{Is…}`, character class subtraction), exclusion, dropped attribute or `code_or_text` entry that no longer matches anything, `code_or_text` attribute that is mandatory or not an enumeration containing the value, or whose element does not end up a single `code` field.

## Node properties and where they come from

Every node of `root` in `spec.gen.json` (Go: `Node` in `usecases/sargen/core/spec.go`) and the other top-level parts of the spec. "constraints.json `x`" = the section `x` of the hand-written file; "generator" = computed by `generate.py` with no input of its own.

| Property | Meaning | Origin |
|---|---|---|
| `name` | Element or attribute name (no `@`); `_text` for the text of an element with simple content and attributes | XSD (`name` of the `xs:element` / `xs:attribute`); `work`, `transactions` and the working fields: constraints.json `working_nodes` |
| `template` | Path of the node, `parent.name` or `parent.name[]` if repeatable; empty for the root | generator (parent template + name, `[]` when `max` is not 1) |
| `kind` | `group` (has children) or `field` (leaf) | generator (a complex type or a `_text` group is a group, a simple type is a field) |
| `min`, `max` | Cardinality, `max` -1 = unbounded | XSD `minOccurs` / `maxOccurs` of the element; attribute: `use="required"` gives 1, otherwise 0, `max` always 1; `_text`: 1 if the base type has a `minLength`, else 0; working nodes: `working_nodes` (`min`, `max`) |
| `attribute` | The node is an XML attribute | XSD (`xs:attribute`) |
| `export` | `false` = working node, never written to the XML | generator (`true` by default); `false` for everything under `work`, from constraints.json `working_nodes` |
| `export_id` | Identifier node that the XML writer builds from internal keys, never holds a value | constraints.json `export_ids` (patterns accept `*`) |
| `type` | `string`, `date`, `decimal`, `integer`, `boolean`, `code` or `ref` | constraints.json `type_map` (simple types and complex types with simple content) and `builtin_types` (`xs:*`); `ref` set by `refs` (except `ref_or_text`); working fields: `type` of the field in `working_nodes` |
| `nature` | Who fills the field: `copied` (absent), `judgment`, `setting`, `computed`, `analyst` | constraints.json `natures` (the longest matching template applies to the whole subtree) |
| `min_length`, `max_length` | Length in characters; for integers and decimals `max_length` is the maximum number of digits | XSD facets `minLength`, `maxLength`, `totalDigits` (of the type, through its restrictions); working fields: `working_nodes` |
| `pattern` | Regex to match entirely | XSD facet `pattern` (anchors stripped for the types of constraints.json `pattern_overrides`); for `xs:positiveInteger`, the generator table BUILTIN_VALUE_PATTERNS (`0*[1-9][0-9]*`); working fields: `working_nodes` |
| `fraction_digits` | Maximum digits after the decimal separator | XSD facet `fractionDigits` |
| `code_list` | Key in `code_lists` of the allowed codes (`code` fields) | XSD `Ref: liste_ref_*.csv` in the documentation of the type, or `xs:enumeration` (list named after the type); union key `a+b` built by the generator when a type cites several references; working fields: `working_nodes` |
| `ref_targets` | Templates of the items a `ref` (or `ref_or_text`) value may designate | constraints.json `refs` (`targets`) |
| `ref_or_text` | The value is the path of a target item or free text; the type stays `string` | constraints.json `refs` (`"ref_or_text": true`) |
| `code_or_text` | The value is a code of `code_list` or, failing that, free text (length and pattern facets apply to both); the type stays `code`; free text is accepted with a `free_text_code` warning | constraints.json `code_or_text` (the XSD attribute, here `type`, of a type, which is then not a node); working fields: `"code_or_text": true` in `working_nodes` |
| `text_attribute` | `{name, value}`: the attribute the XML writer puts on the element when a `code_or_text` value is free text (`type="other"`); absent on working fields | constraints.json `code_or_text` (`attribute`, `value`) |
| `label` | Short name shown to the analyst and the LLM | constraints.json `labels`; `work` and `work.transactions[]`: `working_nodes` (`label`) |
| `description` | Help text shown to the analyst and the LLM | XSD `xs:documentation` of the element or attribute, otherwise of its type (generic length texts dropped through `ignore_type_doc_patterns`); replaced by constraints.json `descriptions` when missing or insufficient; working nodes: `working_nodes` (`description`) |
| `children` | Child nodes: XSD attributes first, then elements in XSD order, then `work` | XSD (`xs:sequence`, `xs:attribute`) minus constraints.json `excluded`, `dropped_attributes` and the `code_or_text` attributes; `_text` and `work` added by the generator |
| `code_lists.<name>.codes` | The `value` / `label` pairs | CSV referential of constraints.json `code_list_bindings` (`file`; columns `code;label`), or the `inline` codes of a binding, or `inline_code_lists`; minus `code_exclusions`; enumeration values of the XSD for an inline enumeration |
| `code_lists.<name>.unavailable` | List not published: any value accepted with a warning | constraints.json `code_list_bindings` (`unavailable`); also set on a union that contains such a list |
| `code_lists.<name>.source` | Where the list comes from | constraints.json `code_list_bindings` (`file ; source`), `inline_code_lists`, `code_exclusions` (appends the excluded codes); union lists: generator |
| `code_lists.<name>.unknown_code` | Code meaning "unknown", taken when the data say nothing | constraints.json `unknown_codes` |
| `code_lists.<name>.other_codes` | Codes meaning "other", for which a free-text precision applies (sorted) | constraints.json `other_codes` |
| `key` | Format key `family`, `country`, `version` | `format` of `manifest.json` (`tracfin:FR:<version>`) |
| `report_types` | Report types handled | constraints.json `report_types` |
| `rules` | Expanded form rules | constraints.json `rules` (`for_each_scope` expanded by the generator); `hand_rules` are not in the spec |
| `defaults` | Raw value set at creation, by template | constraints.json `defaults` |
| `inputs` | Pinned input files (`path`, `sha256`) | `manifest.json` (`inputs`) |

## How each constraint is built

**Tree.** The XSD is read in order: root `ds`, flattened `xs:sequence` (nested ones included), `xs:choice` and `xs:all` make the generator fail (none in XSD 1.4.0; neither the validator nor the XML writer can pick a branch), attributes before elements. An element with simple content and attributes (`numeroIdentification`, `precisionsSuspicion`) becomes a group: its attributes plus a `_text` child for the element text (required, `min` 1, if the base type imposes a minimum length; it then also carries `min_length`). An attribute is an `attribute: true` node, named without `@` (the `name[key].name` path does not accept `@`): `personnes.pp[].referencesFinancieres.referenceFinanciereIban[].iban`. Template of a repeatable child: `parent.name[]`; otherwise `parent.name`. The root has the empty template.

**Cardinalities.** `minOccurs`/`maxOccurs` of the XSD (`unbounded` = -1). Required attribute = `min` 1.

**Types.** `constraints.json` → `type_map` gives the `FieldType` of each simple type (and of complex types with simple content, `t_pays`…). `xs:boolean` → boolean; `t_date` → date (no pattern: stored as YYYY-MM-DD, written as DD/MM/YYYY); integers (`t_num_6`…) → integer; `t_decimal_12` → decimal; types with `Ref:` or `xs:enumeration` → code; the rest → string. The generator checks that "code" matches a list or an enumeration, and conversely.

**Facets.** `maxLength`/`minLength` → `max_length`/`min_length` (strings, in characters). For numeric types `totalDigits` → `max_length` (maximum number of digits, separator excluded) and `fractionDigits` → `fraction_digits`. `pattern` → `pattern`, copied as is (XSD pattern = full match; the validator anchors it). `code` nodes also carry `min_length`, `max_length` and `pattern` of their XSD type, and the validator applies them before list membership (including for an unpublished list). The generator fails if a code of a list does not respect these facets (code `1` "Autre" of `liste_ref_pays.csv` does not have the 2 characters of `t_pays`); the decision to exclude this code is taken in `code_exclusions` (see "Countries").

**Values of builtin types.** `xs:positiveInteger` (`t_num_6`, `t_num_19`, `numero`, `nombreOperations`, `aav.reference`…) excludes 0: the `integer` node receives the pattern `0*[1-9][0-9]*` (BUILTIN_VALUE_PATTERNS in `generate.py`). The generator fails on any builtin type whose value space is not encoded in this table, and on a positive type that would already have its own pattern. `totalDigits` and `fractionDigits` count the digits of the value: leading zeros and trailing zeros of the fraction excluded. The only bounds facet ignored: `minExclusive` of `t_positiveDecimals_32` (`ignored_facets` list), a type that no retained element uses; any other kind of bound makes the generator fail. The `montant` working field carries its own positivity pattern.

**Patterns and regexes.** `pattern_overrides` removes the leading `^` and trailing `$` of `t_num_tel`, `t_mail`, `t_iban` and `t_numCard` (XSD:80-102, 334-368): in XSD regex these signs are literal. Assumption: ERMES reads them as anchors; to be confirmed by an import on ERMES demo. Note: `t_numCard` contains `(?:…)`, valid in Go but not in strict XSD regex. Consumers must anchor each pattern themselves in the form `^(?:` + pattern + `)$` (some patterns are top-level alternations). In RE2 (Go), `\d`, `\s` and `\w` are limited to ASCII: the validator may refuse a Unicode value that the XSD accepts (an accented email address, for example). Accepted limit for v1.

Three differences between the test copy of the XSD (`testdata/financier-v1.4.0-test.xsd`, validated by xmllint) and the official XSD: (1) `^` and `$` removed; (2) `(?:` replaced by `(` in `t_numCard`; (3) in `t_mail`, the class `[\w-\._]` is written `[\w\-\._]`: libxml2 reads `\w-\.` as an invalid range and loses the hyphen, which would reject `jean-pierre.dupont@ma-banque.fr`. The pattern of the spec (`spec.gen.json`) stays the one of the official XSD; Go (RE2) reads it as intended. The Go validator is also more permissive than the XSD on `_` in the domain (Go `\w` includes `_`): accepted.

**Code lists.** Each `Ref: liste_ref_*.csv` of the XSD is bound by `code_list_bindings` to a CSV (code/label columns), to an inline list (`liste_ref_typemime.csv` = `application/pdf`) or marked `unavailable` (`liste_ref_type_equipement.csv`: the list is not published, any value is accepted with a warning). The name of the list in `code_lists` is the name of the `Ref:`; `Node.code_list` carries this key. An inline enumeration becomes a list named after its simple type (`t_registre`). A type that cites two references (`t_pays_dromcom`: DROM-COM and countries) gives the union list `liste_ref_dromcom.csv+liste_ref_pays.csv` (the first code wins). The XSD cites 18 distinct references; `liste_ref_devises.csv` is added to them (`extra`) for the working nodes.

**Unknown code.** `unknown_codes` (`list`, `code`, `source`; any other key makes the generator fail) marks, in a list, the code that means "unknown" (`unknown_code` in `spec.gen.json`). `t_choix_oui_non_nesaispas` and `t_registre` take "Je ne sais pas"; `liste_ref_sexe.csv` takes SEX3 ("Inconnu"); the two country lists (`liste_ref_pays.csv` and the DROM-COM + countries union) take `XX` ("Inconnu", a published line of `pays.csv`). `t_implication` (a judgment field) is deliberately not marked. It is a hand-written decision: the code must exist in the list (the generator fails on a stale entry, an unpublished list or a list declared twice). The agent tells the LLM to take this code when the data say nothing; it needs no source. Every other code of the list keeps the usual proof rules: the code or its label word for word, else a justification and a confidence forced to low.

**Other codes.** `other_codes` (`list`, `codes`, `source`; any other key makes the generator fail) lists, in a list, the codes that mean "other" and for which a free-text precision applies (`other_codes` in `spec.gen.json`, sorted). `liste_ref_relations.csv` lists the "Autre…" codes (TLP11, TLP20, TLP36, TLP47, TLP50, TLP51, TLP56): only they admit `sousCategorieRelationAutre`. It is a hand-written decision: every code must exist in the list (the generator fails on a stale entry, an unpublished list, a duplicate or a list declared twice).

**Countries.** `t_paysName` carries an optional attribute `type` (`code` / `other`): the XSD says to add `type="other"` and write the country in the element to give a country that is not in the list. `t_pays` and `t_pays_dromcom` (length 2..200) restrict it. v1 supports free-text countries through constraints.json `code_or_text` (`type`, `attribute`, `value`, `source`): the attribute is not turned into a node, the element stays one `code` field with `code_or_text: true` and `text_attribute: {name: "type", value: "other"}` (`pays`, `paysNaissance`, `nationalite[]`, `paysAutreTypeRegistre`, `paysOrigineFonds`, `paysArriveeFonds`; `paysDelivrance` belongs to the excluded attachments). The working country fields `paysOrigine` and `paysArrivee` carry `"code_or_text": true` in `working_nodes`. The validator checks the length/pattern facets first (blocking), then a value that is not in the list is accepted with the warning `free_text_code` (params `code_list`, `value`: typo or value really missing from the list?); a listed code raises nothing. The XML writer writes `<nationalite type="other">Kosovo du Nord</nationalite>` for free text and no attribute for a listed code. Code `1` ("Autre") of `liste_ref_pays.csv` stays removed from the country lists and their union by `code_exclusions` (`list`, `codes`, `source`): it is shorter than `minLength` 2 and free text replaces it; an entry that removes nothing makes the generator fail. `dropped_attributes` (`type`, `attribute`, `source`) is still supported, currently empty.

**Exclusions.** `excluded` removes from the spec the elements outside v1, all `minOccurs=0` in the XSD: `operations.operationEnCours[]`, `operationstdf`, `pieceJointe`, `context.general.complement` (complementary declaration only) and the references to attachments (`idPieceJointe`, `idPieceJointeDonneesConnexion`, `infosFinancieresFiscales` which contains nothing else). An exclusion that is mandatory in the XSD or that removes nothing makes the generator fail. Consequence: the rule "TA2/3/5 → infoPieceIdentite" (attachments) does not exist in v1.

**Natures.** `natures`: template → nature; the most specific entry applies to the whole subtree. Absent = copied.

**References and identifiers.** `refs`: the field becomes `type: ref` with `ref_targets`. `export_ids`: identifier elements and attributes that the XML writer builds from internal keys (`personnes.*[].id`, `id` attribute of financial references); they carry `export_id: true`, never have a value in the document, and are neither required by the validator nor proposed to the LLM or the front end. The `export_ids` patterns accept `*` for a segment. The fields `emetteurTypeEmetteur`, `beneficiaireTypeBeneficiaire`, `refFinanciereEmission`, `refFinanciereReception` (and the working nodes `refEmission`, `refReception`) accept the identifier of an already declared person or financial equipment **or** free text: a `refs` entry with `"ref_or_text": true` leaves them as strings (facets kept) and adds `ref_targets` and `ref_or_text: true`. If the value is the path of an existing element whose template is a target, the validator requires the element to exist (`dangling_ref`) and the XML writer replaces it with the element identifier; if the value is the path of an element of the format (keyed segment, known template) whose template is NOT a target, the validator refuses it (`bad_ref`): it is neither an allowed reference nor free text; otherwise it is free text, written as is.

**Default values.** `defaults` → `spec.defaults`: template → raw value set at creation. `{case_id}` is replaced with the case identifier.

**Labels and descriptions.** `labels`: French labels of the sections. `Node.description` = the `xs:documentation` of the element, otherwise that of its type (the generic `Chaines de caractere de taille N` documentations are ignored); `descriptions` replaces the text when it is missing or insufficient.

**Working nodes.** `working_nodes` adds `work` (not exported) after the XSD children: `work.transactions[]` and its fields, all `export: false`, preceded by the optional fields of `working_nodes.root.fields` directly under `work` (`montantCredits` and `montantDebits`, computed totals of the credits and of the debits, and `sensMontantTotal`, the direction the judgment step keeps for the amount at stake; their natures are in `natures`).

**Rules.** `rules`: `id`, `kind` (`required_if`, `forbidden_if`, `exactly_one`, `at_most_one`), `scope` (template of a node), `then`/`among`, `when` (`path`, `op`, `values`), `severity` (absent = blocking; `warning` for the XSD "to fill only if" cases), `description`, `source`. A `for_each_scope` field applies the rule to several scopes (`{scope}` in the templates, identifier suffixed `@<scope>`). All the templates of a rule are absolute; a repeated template must be under the scope, and is resolved relative to the concrete instance of the scope; a template without `[]` is global. Semantics:
- `required_if`: the fields of `then` are required if `when` is true; `forbidden_if`: forbidden if `when` is true;
- `exactly_one` / `at_most_one`: exactly one, or at most one, of the fields of `among` is filled; if `when` is present, the rule applies only when it is true;
- conditions on a repeated path: `eq`/`in` true if one value matches, `not_in` true if no value matches (an empty field included: "forbidden unless X is A" holds while X is empty), `present`/`absent` depending on whether a value exists; `absent` is true when the field is absent.

`hand_rules` lists the rules that cannot be expressed (`id`, `description`, `source`), written in Go by the validator and absent from `spec.gen.json`.

## Notes

- The `source` fields cite only the files committed in `versions/1.4.0/inputs/` (`financier-v1.4.0.xsd:N` line numbers, CSV code lists), TRACFIN publications that are not stored here (`GuideUtilisateurs.pdf`, the `FicheFocus-*.pdf` sheets, the « Le montant en jeu » framing sheet), or a « Design decision » explained in the field itself.
- `.gitattributes` marks `versions/**/inputs/**` as `-text`: the bytes (CRLF line endings included) must stay the published ones, their digests are in `manifest.json`. Never renormalize these files.
- A `hand_rules` code must have its implementation in `HandRuleIDs()` (`validate.go`); a test compares the two lists.
