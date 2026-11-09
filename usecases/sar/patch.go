package sar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/checkmarble/marble-backend/models"
)

// Policy decides which values a patch may change.
type Policy struct {
	// AllowManualOverwrite lets the patch replace analyst values (manual edits
	// and chat requests do; the agent does not).
	AllowManualOverwrite bool
	// AllowAnalysis lets the patch change the protected human analysis.
	AllowAnalysis bool
	// Prefixes restricts the pivot paths (e.g. "pivot/parties"); empty = all.
	Prefixes []string
}

type PatchError struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e PatchError) Error() string { return e.Code + " " + e.Path + ": " + e.Message }

func perr(path, code, format string, args ...any) PatchError {
	return PatchError{Path: path, Code: code, Message: fmt.Sprintf(format, args...)}
}

// Exts holds the extensions of the reports of a case, by format key.
type Exts map[string]models.SarExtension

// ApplyOps applies ops in order; it fails atomically on the first error.
func ApplyOps(p *models.SarPivot, exts Exts, ops []models.SarPatchOp, prov models.SarProvenance, policy Policy) ([]string, error) {
	doc, err := toMap(p)
	if err != nil {
		return nil, err
	}
	var touched []string
	newExts := Exts{}
	for k, v := range exts {
		c := models.SarExtension{}
		for kk, vv := range v {
			c[kk] = vv
		}
		newExts[k] = c
	}
	for i, op := range ops {
		switch {
		case strings.HasPrefix(op.Path, "pivot/"):
			if err := pivotOp(doc, op, prov, policy, i); err != nil {
				return nil, err
			}
		case strings.HasPrefix(op.Path, "extension/"):
			if err := extOp(newExts, op, prov, policy); err != nil {
				return nil, err
			}
		default:
			return nil, perr(op.Path, "PATCH_PATH", "path must start with pivot/ or extension/")
		}
		touched = append(touched, op.Path)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var out models.SarPivot
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return nil, perr("pivot", "PATCH_SHAPE", "%s", err.Error())
	}
	*p = out
	for k, v := range newExts {
		exts[k] = v
	}
	return touched, nil
}

func toMap(p *models.SarPivot) (map[string]any, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(raw, &m)
}

func provMap(prov models.SarProvenance) map[string]any {
	raw, _ := json.Marshal(prov)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

func scalar(raw json.RawMessage) (string, bool) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, true
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String(), true
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return strconv.FormatBool(b), true
	}
	return "", false
}

// rawIDs are kept as plain strings inside pivot objects.
var rawIDs = map[string]bool{"id": true, "file_id": true}

// wrap converts a plain object (strings, lists of strings, nested objects)
// into pivot JSON with SarText values carrying prov.
func wrap(v any, prov map[string]any, prefix string, seq *int) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		if _, isText := x["v"]; isText && len(x) <= 2 {
			return map[string]any{"v": fmt.Sprint(x["v"]), "p": prov}
		}
		for k, val := range x {
			if rawIDs[k] {
				out[k] = fmt.Sprint(val)
				continue
			}
			out[k] = wrap(val, prov, k, seq)
		}
		if _, ok := out["id"]; !ok && prefix != "" {
			*seq++
			out["id"] = fmt.Sprintf("%s%d", idPrefix(prefix), *seq)
		}
		return out
	case []any:
		out := make([]any, 0, len(x))
		for _, e := range x {
			out = append(out, wrap(e, prov, prefix, seq))
		}
		return out
	case nil:
		return nil
	default:
		return map[string]any{"v": fmt.Sprint(x), "p": prov}
	}
}

func idPrefix(collection string) string {
	switch collection {
	case "parties":
		return "p"
	case "instruments":
		return "acc"
	case "links":
		return "l"
	case "operations":
		return "op"
	case "attachments":
		return "att"
	case "addresses":
		return "a"
	case "identifications":
		return "i"
	case "prior_reports":
		return "pr"
	}
	return "x"
}

// NewID derives a stable object ID from a seed.
func NewID(prefix, seed string) string {
	h := sha256.Sum256([]byte(seed))
	return prefix + "_" + hex.EncodeToString(h[:])[:10]
}

func allowed(path string, policy Policy) bool {
	if len(policy.Prefixes) == 0 {
		return true
	}
	for _, p := range policy.Prefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func pivotOp(doc map[string]any, op models.SarPatchOp, prov models.SarProvenance, policy Policy, n int) error {
	if !allowed(op.Path, policy) {
		return perr(op.Path, "PATCH_SCOPE", "path outside the allowed sections %v", policy.Prefixes)
	}
	if strings.HasPrefix(op.Path, "pivot/suspicion/analysis") && !policy.AllowAnalysis {
		return perr(op.Path, "PATCH_PROTECTED", "the analysis is written by the analyst only")
	}
	for _, forbidden := range []string{"pivot/case_id", "pivot/org_id", "pivot/revision", "pivot/validated", "pivot/updated_at", "pivot/updated_by"} {
		if strings.HasPrefix(op.Path, forbidden) {
			return perr(op.Path, "PATCH_PROTECTED", "server-controlled field")
		}
	}
	segs := strings.Split(strings.TrimPrefix(op.Path, "pivot/"), "/")
	return navigate(doc, segs, op, provMap(prov), policy, n)
}

func navigate(m map[string]any, segs []string, op models.SarPatchOp, pm map[string]any, policy Policy, n int) error {
	key := segs[0]
	if len(segs) == 1 {
		return setInMap(m, key, op, pm, policy, n)
	}
	child, ok := m[key]
	if !ok || child == nil {
		if op.Op == "remove" {
			return nil
		}
		return perr(op.Path, "PATCH_NOT_FOUND", "unknown path segment %q", key)
	}
	switch c := child.(type) {
	case map[string]any:
		return navigate(c, segs[1:], op, pm, policy, n)
	case []any:
		idx, ok := findIndex(c, segs[1])
		if !ok {
			if op.Op == "remove" {
				return nil
			}
			return perr(op.Path, "PATCH_NOT_FOUND", "no element %q in %s", segs[1], key)
		}
		if len(segs) == 2 {
			return setInArray(m, key, c, idx, op, pm, policy)
		}
		el, ok := c[idx].(map[string]any)
		if !ok {
			return perr(op.Path, "PATCH_NOT_FOUND", "cannot descend into %q", segs[1])
		}
		return navigate(el, segs[2:], op, pm, policy, n)
	}
	return perr(op.Path, "PATCH_NOT_FOUND", "cannot descend into %q", key)
}

func findIndex(arr []any, key string) (int, bool) {
	for i, e := range arr {
		if m, ok := e.(map[string]any); ok {
			if id, _ := m["id"].(string); id == key {
				return i, true
			}
		}
	}
	if idx, err := strconv.Atoi(key); err == nil && idx >= 0 && idx < len(arr) {
		if m, ok := arr[idx].(map[string]any); !ok || m["id"] == nil {
			return idx, true
		}
	}
	return 0, false
}

func isManual(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	p, ok := m["p"].(map[string]any)
	return ok && p["kind"] == models.SarProvManual
}

func setInMap(m map[string]any, key string, op models.SarPatchOp, pm map[string]any, policy Policy, n int) error {
	cur := m[key]
	if isManual(cur) && !policy.AllowManualOverwrite {
		return perr(op.Path, "PATCH_MANUAL_PROTECTED", "an analyst value cannot be overwritten by the model")
	}
	switch op.Op {
	case "remove":
		delete(m, key)
		return nil
	case "set":
		if arr, isList := cur.([]any); isList || looksList(op.Value) {
			var values []string
			if err := json.Unmarshal(op.Value, &values); err != nil {
				return perr(op.Path, "PATCH_VALUE", "a list of strings is expected")
			}
			for _, e := range arr {
				if isManual(e) && !policy.AllowManualOverwrite {
					return perr(op.Path, "PATCH_MANUAL_PROTECTED", "an analyst value cannot be overwritten by the model")
				}
			}
			out := make([]any, 0, len(values))
			for _, v := range values {
				out = append(out, map[string]any{"v": v, "p": pm})
			}
			m[key] = out
			return nil
		}
		s, ok := scalar(op.Value)
		if !ok {
			return perr(op.Path, "PATCH_VALUE", "a scalar value is expected")
		}
		if s == "" {
			delete(m, key)
			return nil
		}
		m[key] = map[string]any{"v": s, "p": pm}
		return nil
	case "add":
		var v any
		if err := json.Unmarshal(op.Value, &v); err != nil {
			return perr(op.Path, "PATCH_VALUE", "invalid JSON value")
		}
		arr, _ := cur.([]any)
		seq := len(arr) + n*100
		switch x := v.(type) {
		case map[string]any:
			arr = append(arr, wrap(x, pm, key, &seq))
		case []any:
			for _, e := range x {
				s, ok := e.(string)
				if !ok || s == "" {
					return perr(op.Path, "PATCH_VALUE", "a list of non-empty strings is expected")
				}
				arr = append(arr, map[string]any{"v": s, "p": pm})
			}
		default:
			s, ok := scalar(op.Value)
			if !ok || s == "" {
				return perr(op.Path, "PATCH_VALUE", "a non-empty scalar value is expected")
			}
			arr = append(arr, map[string]any{"v": s, "p": pm})
		}
		m[key] = arr
		return nil
	}
	return perr(op.Path, "PATCH_OP", "unknown op %q", op.Op)
}

func looksList(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '['
}

func setInArray(m map[string]any, key string, arr []any, idx int, op models.SarPatchOp, pm map[string]any, policy Policy) error {
	if isManual(arr[idx]) && !policy.AllowManualOverwrite {
		return perr(op.Path, "PATCH_MANUAL_PROTECTED", "an analyst value cannot be overwritten by the model")
	}
	switch op.Op {
	case "remove":
		m[key] = append(arr[:idx:idx], arr[idx+1:]...)
		return nil
	case "set":
		if obj, isObj := arr[idx].(map[string]any); isObj {
			if _, hasID := obj["id"]; hasID {
				var v map[string]any
				if err := json.Unmarshal(op.Value, &v); err != nil {
					return perr(op.Path, "PATCH_VALUE", "an object is expected")
				}
				v["id"] = obj["id"]
				seq := 0
				arr[idx] = wrap(v, pm, key, &seq)
				return nil
			}
		}
		s, ok := scalar(op.Value)
		if !ok {
			return perr(op.Path, "PATCH_VALUE", "a scalar value is expected")
		}
		arr[idx] = map[string]any{"v": s, "p": pm}
		return nil
	}
	return perr(op.Path, "PATCH_OP", "op %q not supported on an element", op.Op)
}

func extOp(exts Exts, op models.SarPatchOp, prov models.SarProvenance, policy Policy) error {
	rest := strings.TrimPrefix(op.Path, "extension/")
	key, field, ok := strings.Cut(rest, "/")
	if !ok {
		return perr(op.Path, "PATCH_PATH", "extension/<format key>/<field> expected")
	}
	a, ok := Get(key)
	if !ok {
		return perr(op.Path, "PATCH_FORMAT", "unknown format %s", key)
	}
	name, _, _ := strings.Cut(field, "#")
	var spec *ExtField
	for _, f := range a.Extension() {
		if f.Key == name {
			f := f
			spec = &f
		}
	}
	if spec == nil {
		return perr(op.Path, "PATCH_FIELD", "unknown extension field %s", name)
	}
	if spec.Computed && !policy.AllowManualOverwrite {
		return perr(op.Path, "PATCH_COMPUTED", "%s is computed by the backend", name)
	}
	if spec.Scope == "settings" {
		return perr(op.Path, "PATCH_SETTINGS", "settings are edited in the configuration")
	}
	ext := exts[key]
	if ext == nil {
		ext = models.SarExtension{}
		exts[key] = ext
	}
	if cur, ok := ext[field]; ok && cur.P != nil && cur.P.Kind == models.SarProvManual && !policy.AllowManualOverwrite {
		return perr(op.Path, "PATCH_MANUAL_PROTECTED", "an analyst value cannot be overwritten by the model")
	}
	switch op.Op {
	case "remove":
		delete(ext, field)
		return nil
	case "set", "add":
		var values []string
		if err := json.Unmarshal(op.Value, &values); err != nil {
			s, ok := scalar(op.Value)
			if !ok {
				return perr(op.Path, "PATCH_VALUE", "a string or a list of strings is expected")
			}
			values = []string{s}
		}
		if op.Op == "add" {
			values = append(append([]string{}, ext[field].Values...), values...)
		}
		p := prov
		ext[field] = models.SarExtValue{Values: values, P: &p}
		return nil
	}
	return perr(op.Path, "PATCH_OP", "unknown op %q", op.Op)
}
