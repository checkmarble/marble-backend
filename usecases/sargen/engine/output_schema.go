package engine

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

const sourceSchemaJSON = `{"anyOf":[{"type":"object","additionalProperties":false,"required":["field","raw_value"],
 "properties":{
  "field":{"type":"string","description":"nom du champ de l'objet source"},
  "raw_value":{"type":"string","description":"valeur brute exacte lue dans l'objet source"},
  "table":{"type":"string","description":"table de l'objet source ; omise = l'objet source unique de l'élément"},
  "object_id":{"type":"string","description":"identifiant de l'objet source ; omis = l'objet source unique de l'élément"}}},
 {"type":"null"}]}`

const fillSchemaJSON = `{"type":"object","additionalProperties":false,"required":["path","value","source"],
 "properties":{
  "path":{"type":"string","description":"chemin du champ à remplir, clés d'éléments existants ou provisoires (ex. personnes.pp[+a].etatCivil.prenom[+a])"},
  "value":{"type":"string","description":"valeur au format du champ (date AAAA-MM-JJ, décimal avec point, code exact)"},
  "source":` + sourceSchemaJSON + `,
  "rule_id":{"anyOf":[{"type":"string"},{"type":"null"}]},
  "justification":{"anyOf":[{"type":"string"},{"type":"null"}],"description":"obligatoire pour un champ de jugement : la donnée lue qui le motive"},
  "confidence":{"anyOf":[{"type":"string","enum":["low","medium","high"]},{"type":"null"}]}}}`

const optionFillSchemaJSON = `{"type":"object","additionalProperties":false,"required":["path","value","source"],
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
  "questions":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["paths","text"],
   "properties":{
    "paths":{"type":"array","items":{"type":"string"}},
    "text":{"type":"string"},
    "options":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["label"],
     "properties":{"label":{"type":"string"},"fills":{"type":"array","items":` + optionFillSchemaJSON + `}}}}}}}}}`

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
//	{fills: [{path, value, source: {field, raw_value, table?, object_id?} | null,
//	          rule_id | null, justification | null,
//	          confidence: "low"|"medium"|"high" | null}],
//	 questions: [{paths, text, options: [{label, fills: [{path, value, source}]}]}]}
//
// path is a free string (provisional `[+a]` keys cannot be enumerated): the
// output control refuses paths outside the task's part. source is compact (spike
// ruling): table and object_id are optional and default to the target item's
// single ItemSource (several sources and none given -> correction error).
// `required` constraints of the format are never described to the LLM.
func FillsOutputSchema() jsonschema.Schema { return mustSchema(fillsSchemaJSON) }

// ReviewOutputSchema is the final answer of a review task:
// {findings: [{paths, severity: "important"|"mineur", message}]}.
func ReviewOutputSchema() jsonschema.Schema { return mustSchema(reviewSchemaJSON) }
