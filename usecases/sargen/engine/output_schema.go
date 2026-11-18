package engine

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

const sourceSchemaJSON = `{"anyOf":[{"type":"object","additionalProperties":false,"required":["field","raw_value","table","object_id"],
 "properties":{
  "field":{"type":"string","description":"name of the field of the source object"},
  "raw_value":{"type":"string","description":"exact raw value read in the source object"},
  "table":{"anyOf":[{"type":"string"},{"type":"null"}],"description":"table of the source object; null = the single source object of the element"},
  "object_id":{"anyOf":[{"type":"string"},{"type":"null"}],"description":"identifier of the source object; null = the single source object of the element"}}},
 {"type":"null"}]}`

const fillSchemaJSON = `{"type":"object","additionalProperties":false,
 "required":["path","value","source","rule_id","justification","confidence"],
 "properties":{
  "path":{"type":"string","description":"path of the field to fill, with existing or provisional element keys (e.g. block.element[+a].group.field)"},
  "value":{"type":"string","description":"value in the field's format (date YYYY-MM-DD, decimal with a dot, exact code)"},
  "source":` + sourceSchemaJSON + `,
  "rule_id":{"anyOf":[{"type":"string"},{"type":"null"}]},
  "justification":{"anyOf":[{"type":"string"},{"type":"null"}],"description":"required for a judgment field: the data read that motivates it"},
  "confidence":{"anyOf":[{"type":"string","enum":["low","medium","high"]},{"type":"null"}]}}}`

const optionFillSchemaJSON = `{"type":"object","additionalProperties":false,
 "required":["path","value","source","rule_id","justification","confidence"],
 "properties":{
  "path":{"type":"string"},
  "value":{"type":"string"},
  "source":` + sourceSchemaJSON + `,
  "rule_id":{"anyOf":[{"type":"string"},{"type":"null"}]},
  "justification":{"anyOf":[{"type":"string"},{"type":"null"}]},
  "confidence":{"anyOf":[{"type":"string","enum":["low","medium","high"]},{"type":"null"}]}}}`

const fillsSchemaJSON = `{"type":"object","additionalProperties":false,"required":["fills","questions"],
 "properties":{
  "fills":{"type":"array","items":` + fillSchemaJSON + `},
  "questions":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["paths","text","options"],
   "properties":{
    "paths":{"type":"array","items":{"type":"string"}},
    "text":{"type":"string"},
    "options":{"anyOf":[{"type":"array","items":{"type":"object","additionalProperties":false,"required":["label","fills"],
     "properties":{"label":{"type":"string"},"fills":{"anyOf":[{"type":"array","items":` + optionFillSchemaJSON + `},{"type":"null"}]}}}},{"type":"null"}]}}}}}}`

const reviewSchemaJSON = `{"type":"object","additionalProperties":false,"required":["findings"],
 "properties":{"findings":{"type":"array","items":{"type":"object","additionalProperties":false,
  "required":["paths","severity","message"],
  "properties":{
   "paths":{"type":"array","items":{"type":"string"}},
   "severity":{"type":"string","enum":["important","mineur"]},
   "message":{"type":"string"}}}}}}`

func mustSchema(s string) jsonschema.Schema {
	var sc jsonschema.Schema
	if err := json.Unmarshal([]byte(s), &sc); err != nil {
		panic(err) // constants above: a programming error
	}
	return sc
}

// FillsOutputSchema is the standard final answer of a fill task:
//
//	{fills: [{path, value, source: {field, raw_value, table | null, object_id | null} | null,
//	          rule_id | null, justification | null,
//	          confidence: "low"|"medium"|"high" | null}],
//	 questions: [{paths, text, options: [{label, fills: [{path, value, source, ...}] | null}] | null}]}
//
// The schema is strict-compatible: every property is listed in `required` and
// the optional ones are nullable (anyOf with null). path is a free string
// (provisional `[+a]` keys cannot be enumerated): the output control refuses
// paths outside the task's part. source is compact (spike ruling): a null table
// and object_id default to the target item's single ItemSource (several sources
// and none given -> correction error). What the format requires of a document
// (required fields) is not described by this schema: DescribeSubtree lists it.
//
// V1 provider support: Gemini through aistudio/Vertex, which applies the
// response schema alongside the tools. The anthropic adapter ignores response
// schemas: it would answer free text, which the output control then corrects.
func FillsOutputSchema() jsonschema.Schema { return mustSchema(fillsSchemaJSON) }

// ReviewOutputSchema is the final answer of a review task:
// {findings: [{paths, severity: "important"|"mineur", message}]} (all
// properties required, strict-compatible).
func ReviewOutputSchema() jsonschema.Schema { return mustSchema(reviewSchemaJSON) }
