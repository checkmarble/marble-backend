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

The script resolves `versions/<version>/` from its own location. Deterministic output: sorted keys, `indent=1`, `\n` line endings, no timestamp or absolute path. It fails (exit code 1, message in English) on: digest mismatch, input file missing from the manifest, simple type missing from `type_map`, list reference without a binding, XSD regex construct that Go cannot read (`\i`, `\c`, `\p{Is…}`, character class subtraction), exclusion or dropped attribute that no longer matches anything.

## How each constraint is built

**Tree.** The XSD is read in order: root `ds`, flattened `xs:sequence` (nested ones included), `xs:choice` and `xs:all` make the generator fail (none in XSD 1.4.0; neither the validator nor the XML writer can pick a branch), attributes before elements. An element with simple content and attributes (`numeroIdentification`, `precisionsSuspicion`) becomes a group: its attributes plus a `_text` child for the element text (required, `min` 1, if the base type imposes a minimum length; it then also carries `min_length`). An attribute is an `attribute: true` node, named without `@` (the `name[key].name` path does not accept `@`): `personnes.pp[].referencesFinancieres.referenceFinanciereIban[].iban`. Template of a repeatable child: `parent.name[]`; otherwise `parent.name`. The root has the empty template.

**Cardinalities.** `minOccurs`/`maxOccurs` of the XSD (`unbounded` = -1). Required attribute = `min` 1.

**Types.** `constraints.json` → `type_map` gives the `FieldType` of each simple type (and of complex types with simple content, `t_pays`…). `xs:boolean` → boolean; `t_date` → date (no pattern: stored as YYYY-MM-DD, written as DD/MM/YYYY); integers (`t_num_6`…) → integer; `t_decimal_12` → decimal; types with `Ref:` or `xs:enumeration` → code; the rest → string. The generator checks that "code" matches a list or an enumeration, and conversely.

**Facets.** `maxLength`/`minLength` → `max_length`/`min_length` (strings, in characters). For numeric types `totalDigits` → `max_length` (maximum number of digits, separator excluded) and `fractionDigits` → `fraction_digits`. `pattern` → `pattern`, copied as is (XSD pattern = full match; the validator anchors it). `code` nodes also carry `min_length`, `max_length` and `pattern` of their XSD type, and the validator applies them before list membership (including for an unpublished list). The generator fails if a code of a list does not respect these facets (code `1` "Autre" of `liste_ref_pays.csv` does not have the 2 characters of `t_pays`); the decision to exclude this code is taken in `code_exclusions` (see "Countries").

**Values of builtin types.** `xs:positiveInteger` (`t_num_6`, `t_num_19`, `numero`, `nombreOperations`, `aav.reference`…) excludes 0: the `integer` node receives the pattern `0*[1-9][0-9]*` (BUILTIN_VALUE_PATTERNS in `generate.py`). The generator fails on any builtin type whose value space is not encoded in this table, and on a positive type that would already have its own pattern. `totalDigits` and `fractionDigits` count the digits of the value: leading zeros and trailing zeros of the fraction excluded. The only bounds facet ignored: `minExclusive` of `t_positiveDecimals_32` (`ignored_facets` list), a type that no retained element uses; any other kind of bound makes the generator fail. The `montant` working field carries its own positivity pattern.

**Patterns and regexes.** `pattern_overrides` removes the leading `^` and trailing `$` of `t_num_tel`, `t_mail`, `t_iban` and `t_numCard` (XSD:80-102, 334-368): in XSD regex these signs are literal. Assumption: ERMES reads them as anchors; to be confirmed by an import on ERMES demo. Note: `t_numCard` contains `(?:…)`, valid in Go but not in strict XSD regex. Consumers must anchor each pattern themselves in the form `^(?:` + pattern + `)$` (some patterns are top-level alternations). In RE2 (Go), `\d`, `\s` and `\w` are limited to ASCII: the validator may refuse a Unicode value that the XSD accepts (an accented email address, for example). Accepted limit for v1.

Three differences between the test copy of the XSD (`testdata/financier-v1.4.0-test.xsd`, validated by xmllint) and the official XSD: (1) `^` and `$` removed; (2) `(?:` replaced by `(` in `t_numCard`; (3) in `t_mail`, the class `[\w-\._]` is written `[\w\-\._]`: libxml2 reads `\w-\.` as an invalid range and loses the hyphen, which would reject `jean-pierre.dupont@ma-banque.fr`. The pattern of the spec (`spec.gen.json`) stays the one of the official XSD; Go (RE2) reads it as intended. The Go validator is also more permissive than the XSD on `_` in the domain (Go `\w` includes `_`): accepted.

**Code lists.** Each `Ref: liste_ref_*.csv` of the XSD is bound by `code_list_bindings` to a CSV (code/label columns), to an inline list (`liste_ref_typemime.csv` = `application/pdf`) or marked `unavailable` (`liste_ref_type_equipement.csv`: the list is not published, any value is accepted with a warning). The name of the list in `code_lists` is the name of the `Ref:`; `Node.code_list` carries this key. An inline enumeration becomes a list named after its simple type (`t_registre`). A type that cites two references (`t_pays_dromcom`: DROM-COM and countries) gives the union list `liste_ref_dromcom.csv+liste_ref_pays.csv` (the first code wins). The XSD cites 18 distinct references; `liste_ref_devises.csv` is added to them (`extra`) for the working nodes.

**Countries.** `t_pays` carries an optional attribute `type="other"` for a free-text country; v1 does not handle it (`dropped_attributes`): the country is always a code. Code `1` ("Autre") of `liste_ref_pays.csv` does not fit in `t_pays` / `t_pays_dromcom` (`minLength` 2) and precisely requires `type="other"` with a free-text country: v1 decision, it is removed from the country lists and from their union by `code_exclusions` (`list`, `codes`, `source`), and the validator refuses it (`unknown_code`). A `code_exclusions` entry that removes nothing makes the generator fail.

**Exclusions.** `excluded` removes from the spec the elements outside v1, all `minOccurs=0` in the XSD: `operations.operationEnCours[]`, `operationstdf`, `pieceJointe`, `context.general.complement` (complementary declaration only) and the references to attachments (`idPieceJointe`, `idPieceJointeDonneesConnexion`, `infosFinancieresFiscales` which contains nothing else). An exclusion that is mandatory in the XSD or that removes nothing makes the generator fail. Consequence: the rule "TA2/3/5 → infoPieceIdentite" (attachments) does not exist in v1.

**Natures.** `natures`: template → nature; the most specific entry applies to the whole subtree. Absent = copied.

**References and identifiers.** `refs`: the field becomes `type: ref` with `ref_targets`. `export_ids`: identifier elements and attributes that the XML writer builds from internal keys (`personnes.*[].id`, `id` attribute of financial references); they carry `export_id: true`, never have a value in the document, and are neither required by the validator nor proposed to the LLM or the front end. The `export_ids` patterns accept `*` for a segment. The fields `emetteurTypeEmetteur`, `beneficiaireTypeBeneficiaire`, `refFinanciereEmission`, `refFinanciereReception` (and the working nodes `refEmission`, `refReception`) accept the identifier of an already declared person or financial equipment **or** free text: a `refs` entry with `"ref_or_text": true` leaves them as strings (facets kept) and adds `ref_targets` and `ref_or_text: true`. If the value is the path of an existing element whose template is a target, the validator requires the element to exist (`dangling_ref`) and the XML writer replaces it with the element identifier; if the value is the path of an element of the format (keyed segment, known template) whose template is NOT a target, the validator refuses it (`bad_ref`): it is neither an allowed reference nor free text; otherwise it is free text, written as is.

**Default values.** `defaults` → `spec.defaults`: template → raw value set at creation. `{case_id}` is replaced with the case identifier.

**Labels and descriptions.** `labels`: French labels of the sections. `Node.description` = the `xs:documentation` of the element, otherwise that of its type (the generic `Chaines de caractere de taille N` documentations are ignored); `descriptions` replaces the text when it is missing or insufficient.

**Working nodes.** `working_nodes` adds `work` (not exported) after the XSD children: `work.transactions[]` and its fields, all `export: false`.

**Rules.** `rules`: `id`, `kind` (`required_if`, `forbidden_if`, `exactly_one`, `at_most_one`), `scope` (template of a node), `then`/`among`, `when` (`path`, `op`, `values`), `severity` (absent = blocking; `warning` for the XSD "to fill only if" cases), `description`, `source`. A `for_each_scope` field applies the rule to several scopes (`{scope}` in the templates, identifier suffixed `@<scope>`). All the templates of a rule are absolute; a repeated template must be under the scope, and is resolved relative to the concrete instance of the scope; a template without `[]` is global. Semantics:
- `required_if`: the fields of `then` are required if `when` is true; `forbidden_if`: forbidden if `when` is true;
- `exactly_one` / `at_most_one`: exactly one, or at most one, of the fields of `among` is filled; if `when` is present, the rule applies only when it is true;
- conditions on a repeated path: `eq`/`in` true if one value matches, `not_in` true if no value matches, `present`/`absent` depending on whether a value exists; `absent` and `not_in` are true when the field is absent.

`hand_rules` lists the rules that cannot be expressed (`id`, `description`, `source`), written in Go by the validator and absent from `spec.gen.json`.

## Notes

- The `analysis.md` citations and the `docs/sar-formats` sheets in the `source` fields are unpublished internal notes not in this repository; the `financier-v1.4.0.xsd:N` line numbers, on the other hand, refer to the XSD committed in `versions/1.4.0/inputs/`.
- `.gitattributes` marks `versions/**/inputs/**` as `-text`: the bytes (CRLF line endings included) must stay the published ones, their digests are in `manifest.json`. Never renormalize these files.
- A `hand_rules` code must have its implementation in `HandRuleIDs()` (`validate.go`); a test compares the two lists.
