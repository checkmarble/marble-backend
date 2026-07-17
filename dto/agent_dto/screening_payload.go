package agent_dto

import (
	"encoding/json"
	"strings"
)

// Maximum nesting depth of entities kept in full. The matched entity is depth 0;
// its directly attached records (Sanction, Identification, Address, or a
// relationship edge such as Family/Associate) are depth 1 and keep their own
// scalar fields; anything nested deeper — typically the profile of a related
// party reached through a relationship edge — is collapsed to base identifying
// info (id, caption, schema, name).
const screeningMaxEntityDepth = 1

// baseEntityFields are the identifying fields kept for a collapsed nested entity.
var baseEntityFields = []string{"id", "caption", "schema"}

// SanitizeScreeningPayloadForLLM trims a FollowTheMoney screening entity payload
// before it is handed to the AI agent for review.
//
// Richer providers (e.g. LexisNexis) return entities carrying long lists of
// referent IDs, many link URLs, and deeply nested relationship graphs. Feeding
// those verbatim into the review prompt inflates the context past the model's
// token limit without adding analytical signal. This function:
//   - drops "referents" (the aggregated list of source-record IDs) at every level,
//   - strips link/URL values from properties (e.g. sourceUrl, programUrl), and
//   - collapses relatives — the profiles of parties reached through relationship
//     edges — to their base identifying info, while keeping the matched entity's
//     own records (sanctions, identifications, ...) intact.
//
// The stored payload is never modified; sanitization applies only to the copy
// sent to the LLM. If the payload is not a JSON object, it is returned unchanged.
func SanitizeScreeningPayloadForLLM(payload []byte) json.RawMessage {
	if len(payload) == 0 {
		return payload
	}

	var entity map[string]any
	if err := json.Unmarshal(payload, &entity); err != nil {
		return payload
	}

	sanitizeEntity(entity, 0)

	out, err := json.Marshal(entity)
	if err != nil {
		return payload
	}
	return out
}

// sanitizeEntity rewrites an FTM entity in place: it removes "referents" and then
// walks "properties", stripping link values and recursing into nested entities.
func sanitizeEntity(entity map[string]any, depth int) {
	delete(entity, "referents")

	props, ok := entity["properties"].(map[string]any)
	if !ok {
		return
	}

	for name, raw := range props {
		values, ok := raw.([]any)
		if !ok {
			delete(props, name)
			continue
		}

		filtered := make([]any, 0, len(values))
		for _, v := range values {
			switch val := v.(type) {
			case string:
				if isLinkValue(val) {
					continue // strip links: many tokens, no analytical signal
				}
				filtered = append(filtered, val)
			case map[string]any:
				filtered = append(filtered, reduceNestedEntity(val, depth+1))
			default:
				filtered = append(filtered, val)
			}
		}

		if len(filtered) == 0 {
			delete(props, name)
			continue
		}
		props[name] = filtered
	}
}

// reduceNestedEntity keeps a nested entity in full (minus referents/links) while
// it is within screeningMaxEntityDepth, and collapses it to base identifying info
// plus its name once past that depth — the point at which we are looking at a
// related party's own profile rather than a record of the matched entity.
func reduceNestedEntity(entity map[string]any, depth int) map[string]any {
	if depth > screeningMaxEntityDepth {
		reduced := baseEntity(entity)
		if props, ok := entity["properties"].(map[string]any); ok {
			if name, ok := props["name"]; ok {
				reduced["properties"] = map[string]any{"name": name}
			}
		}
		return reduced
	}

	reduced := baseEntity(entity)
	if props, ok := entity["properties"].(map[string]any); ok {
		reduced["properties"] = props
		sanitizeEntity(reduced, depth)
	}
	return reduced
}

func baseEntity(entity map[string]any) map[string]any {
	reduced := make(map[string]any, len(baseEntityFields)+1)
	for _, key := range baseEntityFields {
		if v, ok := entity[key]; ok {
			reduced[key] = v
		}
	}
	return reduced
}

func isLinkValue(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
