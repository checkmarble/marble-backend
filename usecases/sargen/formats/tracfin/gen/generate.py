"""Deterministic generator of the TRACFIN format description (spec.gen.json).

Usage: python3 -I generate.py --version 1.4.0 --out <file>

Reads versions/<version>/ (resolved from this file's location, never from the
working directory): manifest.json, constraints.json and the pinned inputs
(XSD + reference CSV files). Python standard library only. The inputs are
untrusted data: they are parsed, never executed.

Same inputs, same bytes: keys are sorted, no timestamp, no absolute path.
"""

import argparse
import csv
import hashlib
import json
import os
import re
import sys
import xml.etree.ElementTree as ET

XS = "{http://www.w3.org/2001/XMLSchema}"
FIELD_TYPES = {"string", "date", "decimal", "integer", "boolean", "code"}
RULE_KINDS = {"required_if", "forbidden_if", "exactly_one", "at_most_one"}
NATURES = {"copied", "judgment", "setting", "computed", "analyst"}
CONDITION_OPS = {"eq", "in", "not_in", "present", "absent"}
REF_START_RE = re.compile(r"[Rr]ef\s*:")
REF_NAME_RE = re.compile(r"liste_ref_[A-Za-z0-9_]+\.csv")

# Builtin XSD types and how the validator encodes their value space. A builtin
# whose value space is not encoded here makes the generator fail: it would be
# silently looser than the XSD.
#   None  = the plain field type (boolean, string, decimal, integer) is exact;
#   other = a pattern (full match) that restricts the integer value space.
BUILTIN_VALUE_PATTERNS = {
    "xs:boolean": None,
    "xs:decimal": None,
    "xs:integer": None,
    "xs:string": None,
    "xs:positiveInteger": "0*[1-9][0-9]*",
}


def refs_in(doc):
    """Reference list names cited after a `Ref:` marker, in order, without duplicates."""
    match = REF_START_RE.search(doc)
    if match is None:
        return []
    out = []
    for name in REF_NAME_RE.findall(doc[match.end():]):
        if name not in out:
            out.append(name)
    return out


class GenError(Exception):
    pass


def fail(message):
    raise GenError(message)


def tag(element):
    return element.tag.replace(XS, "")


def norm(text):
    return " ".join(text.split())


# --------------------------------------------------------------------------
# Inputs
# --------------------------------------------------------------------------


def load_json(path):
    with open(path, "r", encoding="utf-8") as handle:
        return json.load(handle)


def verify_inputs(version_dir, manifest):
    """Check every pinned input against its sha256; reject unlisted files."""
    listed = set()
    for entry in manifest["inputs"]:
        path = os.path.join(version_dir, entry["path"])
        listed.add(os.path.normpath(path))
        if not os.path.isfile(path):
            fail("input not found: %s" % entry["path"])
        with open(path, "rb") as handle:
            digest = hashlib.sha256(handle.read()).hexdigest()
        if digest != entry["sha256"]:
            fail("digest mismatch for %s: expected %s, computed %s" % (entry["path"], entry["sha256"], digest))
    inputs_dir = os.path.join(version_dir, "inputs")
    for root, _dirs, files in os.walk(inputs_dir):
        for name in files:
            if os.path.normpath(os.path.join(root, name)) not in listed:
                fail("input file missing from the manifest: %s" % os.path.join(root, name))


def read_codes(version_dir, manifest, binding, list_name):
    path = binding["file"]
    encoding = None
    for entry in manifest["inputs"]:
        if entry["path"] == path:
            encoding = entry["encoding"]
    if encoding is None:
        fail("list %s: %s missing from the manifest" % (list_name, path))
    codes = []
    seen = set()
    with open(os.path.join(version_dir, path), "r", encoding=encoding, newline="") as handle:
        reader = csv.DictReader(handle, delimiter=";")
        for column in (binding["code_column"], binding["label_column"]):
            if column not in (reader.fieldnames or []):
                fail("list %s: column %r missing from %s" % (list_name, column, path))
        for row in reader:
            value = (row[binding["code_column"]] or "").strip()
            label = (row[binding["label_column"]] or "").strip()
            if not value and not label:
                continue
            if not value:
                fail("list %s: empty code in %s" % (list_name, path))
            if value in seen:
                fail("list %s: duplicate code %r in %s" % (list_name, value, path))
            seen.add(value)
            codes.append({"value": value, "label": label})
    if not codes:
        fail("list %s: no code in %s" % (list_name, path))
    return codes


# --------------------------------------------------------------------------
# XSD regular expressions
# --------------------------------------------------------------------------


def check_regex(pattern, where):
    """Fail on XSD regex constructs that Go (RE2) cannot read."""
    i = 0
    in_class = False
    while i < len(pattern):
        char = pattern[i]
        if char == "\\":
            if i + 1 >= len(pattern):
                fail("%s: pattern ends with a backslash" % where)
            nxt = pattern[i + 1]
            if nxt in "iIcC":
                fail("%s: unsupported XSD construct \\%s in %r (handle it in pattern_overrides)" % (where, nxt, pattern))
            if nxt in "pP" and pattern[i + 2 : i + 4] == "{I":
                fail("%s: unsupported XSD Unicode block in %r" % (where, pattern))
            i += 2
            continue
        if in_class:
            if char == "-" and pattern[i + 1 : i + 2] == "[":
                fail("%s: unsupported XSD character class subtraction in %r (handle it in pattern_overrides)" % (where, pattern))
            if char == "]":
                in_class = False
        elif char == "[":
            in_class = True
            if pattern[i + 1 : i + 2] == "^":
                i += 1
            if pattern[i + 1 : i + 2] == "]":
                i += 1
        i += 1


def strip_anchors(pattern):
    if pattern.startswith("^"):
        pattern = pattern[1:]
    if pattern.endswith("$") and not pattern.endswith("\\$"):
        pattern = pattern[:-1]
    return pattern


# --------------------------------------------------------------------------
# XSD model
# --------------------------------------------------------------------------


def documentation(element):
    """Normalised xs:documentation text directly attached to element."""
    annotation = element.find(XS + "annotation")
    if annotation is None:
        return ""
    parts = []
    for doc in annotation.findall(XS + "documentation"):
        parts.append(norm("".join(doc.itertext())))
    return norm(" ".join(p for p in parts if p))


def strip_prefix(name):
    return name.split(":", 1)[1] if ":" in name else name


class Generator:
    def __init__(self, version, version_dir):
        self.version = version
        self.dir = version_dir
        self.manifest = load_json(os.path.join(version_dir, "manifest.json"))
        self.constraints = load_json(os.path.join(version_dir, "constraints.json"))
        verify_inputs(version_dir, self.manifest)

        xsd_path = None
        for entry in self.manifest["inputs"]:
            if entry["path"].endswith(".xsd"):
                if xsd_path is not None:
                    fail("several XSDs in the manifest")
                xsd_path = os.path.join(version_dir, entry["path"])
        if xsd_path is None:
            fail("no XSD in the manifest")
        self.xsd_name = os.path.basename(xsd_path)
        root = ET.parse(xsd_path).getroot()
        for pattern_el in root.iter(XS + "pattern"):
            check_regex(pattern_el.get("value", ""), "XSD (all patterns are checked)")

        self.simple_types = {}
        self.complex_types = {}
        self.top_elements = {}
        for child in root:
            name = child.get("name")
            if tag(child) == "simpleType":
                self.simple_types[name] = child
            elif tag(child) == "complexType":
                self.complex_types[name] = child
            elif tag(child) == "element":
                self.top_elements[name] = child

        self.type_map = self.constraints["type_map"]["types"]
        self.builtin_map = self.constraints["builtin_types"]["types"]
        for table in (self.type_map, self.builtin_map):
            for name, kind in table.items():
                if kind not in FIELD_TYPES:
                    fail("type %s: unknown field type %r" % (name, kind))
        self.ignore_doc = [re.compile(p) for p in self.constraints["ignore_type_doc_patterns"]["patterns"]]
        self.ignored_facets = set(self.constraints["ignored_facets"]["facets"])
        self.pattern_overrides = self.constraints["pattern_overrides"]
        self.dropped = {}
        for entry in self.constraints["dropped_attributes"]:
            self.dropped[(entry["type"], entry["attribute"])] = False
        self.code_or_text = {}
        for entry in self.constraints.get("code_or_text", []):
            require_source(entry, "code_or_text[%s.%s]" % (entry.get("type"), entry.get("attribute")))
            unexpected = sorted(set(entry) - {"type", "attribute", "value", "source"})
            if unexpected:
                fail("code_or_text: unexpected keys %s" % ", ".join(unexpected))
            key = (entry["type"], entry["attribute"])
            if key in self.dropped or key in self.code_or_text:
                fail("code_or_text: %s.%s declared twice (also in dropped_attributes?)" % key)
            self.code_or_text[key] = {"value": entry["value"], "used": False}
        self.excluded_templates = {}
        self.excluded_names = {}
        for entry in self.constraints["excluded"]:
            require_source(entry, "excluded")
            if "template" in entry:
                self.excluded_templates[entry["template"]] = False
            else:
                self.excluded_names[entry["name"]] = False

        self.code_lists = {}
        self.inline_used = {}
        self.check_all_types_mapped()
        self.check_bindings()
        self.build_bound_lists()

    # -- type checks ---------------------------------------------------------

    def check_all_types_mapped(self):
        for name in sorted(self.simple_types):
            if name not in self.type_map:
                fail("simple type %s of the XSD missing from constraints.json type_map: decide its type before generating" % name)
        for name in sorted(self.complex_types):
            if self.complex_text_base(name) is not None and name not in self.type_map:
                fail("complex type with simple content %s of the XSD missing from constraints.json type_map" % name)
        for name in sorted(self.type_map):
            if name not in self.simple_types and name not in self.complex_types:
                fail("type_map: %s does not exist in the XSD" % name)

    def complex_text_base(self, name):
        """Return the xs:simpleContent derivation of a complex type, or None."""
        ct = self.complex_types[name]
        sc = ct.find(XS + "simpleContent")
        if sc is None:
            return None
        return sc

    def check_bindings(self):
        found = set()
        for source in list(self.simple_types.values()) + list(self.complex_types.values()):
            for ref in refs_in(documentation(source)):
                found.add(ref)
        bindings = self.constraints["code_list_bindings"]
        for ref in sorted(found):
            if ref not in bindings:
                fail("XSD reference %s has no binding in code_list_bindings" % ref)
        for name, binding in sorted(bindings.items()):
            require_source(binding, "code_list_bindings[%s]" % name)
            if name not in found and not binding.get("extra"):
                fail("code_list_bindings: %s is not referenced anywhere in the XSD (mark it extra if intended)" % name)

    def build_bound_lists(self):
        for name, binding in sorted(self.constraints["code_list_bindings"].items()):
            lst = {"name": name, "source": binding["source"]}
            if binding.get("unavailable"):
                lst["unavailable"] = True
            elif "inline" in binding:
                lst["codes"] = [dict(c) for c in binding["inline"]]
            else:
                lst["codes"] = read_codes(self.dir, self.manifest, binding, name)
                lst["source"] = "%s ; %s" % (binding["file"], binding["source"])
            self.code_lists[name] = lst
        self.apply_code_exclusions()
        for name, entry in sorted(self.constraints["inline_code_lists"].items()):
            require_source(entry, "inline_code_lists[%s]" % name)
            self.code_lists[name] = {"name": name, "codes": [dict(c) for c in entry["codes"]], "source": entry["source"]}

    def apply_code_exclusions(self):
        """Remove from a bound list the codes that v1 cannot express (decision in constraints.json)."""
        for entry in self.constraints.get("code_exclusions", []):
            require_source(entry, "code_exclusions[%s]" % entry.get("list"))
            lst = self.code_lists.get(entry["list"])
            if lst is None or lst.get("unavailable"):
                fail("code_exclusions: list %s not found or not published" % entry["list"])
            for code in entry["codes"]:
                if code not in [c["value"] for c in lst["codes"]]:
                    fail("code_exclusions: code %r is not in %s (stale entry)" % (code, entry["list"]))
            lst["codes"] = [c for c in lst["codes"] if c["value"] not in entry["codes"]]
            lst["source"] += " ; excluded codes: " + ", ".join(entry["codes"])

    def apply_unknown_codes(self):
        """Mark the code that means "unknown" in the lists that have one (decision in constraints.json)."""
        for entry in self.constraints.get("unknown_codes", []):
            unexpected = sorted(set(entry) - {"list", "code", "source"})
            if unexpected:
                fail("unknown_codes[%s]: unexpected keys %s" % (entry.get("list"), ", ".join(unexpected)))
            require_source(entry, "unknown_codes[%s]" % entry.get("list"))
            lst = self.code_lists.get(entry["list"])
            if lst is None or lst.get("unavailable"):
                fail("unknown_codes: list %s not found or not published" % entry["list"])
            if entry["code"] not in [c["value"] for c in lst["codes"]]:
                fail("unknown_codes: code %r is not in %s (stale entry)" % (entry["code"], entry["list"]))
            if "unknown_code" in lst:
                fail("unknown_codes: list %s declared twice" % entry["list"])
            lst["unknown_code"] = entry["code"]

    def apply_other_codes(self):
        """Mark the "other" codes of the lists that have some (decision in constraints.json)."""
        for entry in self.constraints.get("other_codes", []):
            unexpected = sorted(set(entry) - {"list", "codes", "source"})
            if unexpected:
                fail("other_codes[%s]: unexpected keys %s" % (entry.get("list"), ", ".join(unexpected)))
            require_source(entry, "other_codes[%s]" % entry.get("list"))
            lst = self.code_lists.get(entry["list"])
            if lst is None or lst.get("unavailable"):
                fail("other_codes: list %s not found or not published" % entry["list"])
            if not entry.get("codes"):
                fail("other_codes: list %s has no code" % entry["list"])
            published = [c["value"] for c in lst["codes"]]
            for code in entry["codes"]:
                if code not in published:
                    fail("other_codes: code %r is not in %s (stale entry)" % (code, entry["list"]))
            if len(set(entry["codes"])) != len(entry["codes"]):
                fail("other_codes: duplicate code in %s" % entry["list"])
            if "other_codes" in lst:
                fail("other_codes: list %s declared twice" % entry["list"])
            lst["other_codes"] = sorted(entry["codes"])

    def merged_list(self, refs):
        """Code list key for a type bound to several references (union, first wins)."""
        if len(refs) == 1:
            return refs[0]
        key = "+".join(refs)
        if key not in self.code_lists:
            codes = []
            seen = set()
            unavailable = False
            for ref in refs:
                src = self.code_lists[ref]
                if src.get("unavailable"):
                    unavailable = True
                for code in src.get("codes", []):
                    if code["value"] not in seen:
                        seen.add(code["value"])
                        codes.append(dict(code))
            lst = {"name": key, "source": "union of " + ", ".join(refs)}
            if unavailable:
                lst["unavailable"] = True
            else:
                lst["codes"] = codes
            self.code_lists[key] = lst
        return key

    # -- simple types --------------------------------------------------------

    def simple_info(self, type_name):
        """Resolve a named simple type (or xs: builtin) to its effective facets."""
        if type_name.startswith("xs:"):
            if type_name not in self.builtin_map:
                fail("XSD builtin type %s missing from constraints.json builtin_types" % type_name)
            if type_name not in BUILTIN_VALUE_PATTERNS:
                fail("XSD builtin type %s: value space not encoded by the generator (BUILTIN_VALUE_PATTERNS)" % type_name)
            return {"kind": self.builtin_map[type_name], "facets": {}, "refs": [], "enum": None, "enum_type": None, "doc": "", "pattern_type": None, "value_pattern": BUILTIN_VALUE_PATTERNS[type_name]}
        if type_name not in self.simple_types:
            fail("unknown simple type: %s" % type_name)
        el = self.simple_types[type_name]
        restriction = el.find(XS + "restriction")
        if restriction is None:
            fail("type %s: only xs:restriction is supported" % type_name)
        info = self.restriction_info(restriction, type_name, self.type_map[type_name], documentation(el))
        return info

    def restriction_info(self, restriction, owner, kind, own_doc, base_info=None):
        base = restriction.get("base")
        if base is None:
            fail("%s: restriction without base" % owner)
        info = dict(base_info if base_info is not None else self.simple_info(base))
        info["facets"] = dict(info["facets"])
        info["kind"] = kind
        refs = refs_in(own_doc)
        if refs:
            info["refs"] = refs
        if own_doc and not any(p.search(owner) for p in self.ignore_doc):
            info["doc"] = own_doc
        enums = []
        for facet in restriction:
            name = tag(facet)
            if name == "annotation":
                continue
            if name == "enumeration":
                enums.append(facet.get("value"))
            elif name == "pattern":
                if "pattern" in info["facets"] and info["pattern_type"] != owner:
                    fail("%s: patterns accumulated over two type levels, not supported" % owner)
                if info["pattern_type"] == owner:
                    fail("%s: several patterns in a single restriction, not supported" % owner)
                info["facets"]["pattern"] = facet.get("value")
                info["pattern_type"] = owner
            elif name in ("maxLength", "minLength", "totalDigits", "fractionDigits"):
                info["facets"][name] = int(facet.get("value"))
            elif name in self.ignored_facets:
                continue
            else:
                fail("%s: unsupported XSD facet %s" % (owner, name))
        if enums:
            info["enum"] = enums
            info["enum_type"] = owner
        return info

    def complex_text_info(self, type_name):
        """Resolve a complex type with xs:simpleContent to (info, attributes)."""
        ct = self.complex_types[type_name]
        sc = ct.find(XS + "simpleContent")
        derivation = sc.find(XS + "extension")
        is_restriction = False
        if derivation is None:
            derivation = sc.find(XS + "restriction")
            is_restriction = True
        if derivation is None:
            fail("%s: simpleContent without extension or restriction" % type_name)
        base = derivation.get("base")
        if base in self.complex_types:
            info, attrs = self.complex_text_info(base)
        else:
            info, attrs = self.simple_info(base), []
        attrs = list(attrs)
        own_doc = documentation(ct)
        if is_restriction:
            info = self.restriction_info(derivation, type_name, self.type_map[type_name], own_doc, info)
        else:
            info = dict(info)
            info["kind"] = self.type_map[type_name]
            refs = refs_in(own_doc)
            if refs:
                info["refs"] = refs
        for attr in derivation.findall(XS + "attribute"):
            attrs.append((type_name, attr))
        return info, attrs

    def inline_simple_info(self, simple_type_el, owner):
        restriction = simple_type_el.find(XS + "restriction")
        if restriction is None:
            fail("%s: anonymous simpleType without restriction" % owner)
        base = restriction.get("base")
        info = self.restriction_info(restriction, owner, self.simple_info(base)["kind"], "")
        return info

    # -- nodes ---------------------------------------------------------------

    def leaf_from_info(self, info, name, template, mn, mx, attribute, doc, type_name):
        kind = info["kind"]
        node = base_node(name, template, "field", mn, mx)
        if attribute:
            node["attribute"] = True
        node["type"] = kind
        facets = info["facets"]
        has_list = bool(info["refs"]) or bool(info["enum"])
        if kind == "code":
            if not has_list:
                fail("%s: type %s is declared code but has no reference list or enumeration" % (template, type_name))
            if info["refs"]:
                node["code_list"] = self.merged_list(info["refs"])
            else:
                list_name = info["enum_type"]
                self.add_inline_enum(list_name, info["enum"])
                node["code_list"] = list_name
        elif has_list:
            fail("%s: type %s has a list or an enumeration but is not declared code" % (template, type_name))
        if kind in ("string", "code"):
            if facets.get("minLength", 0) > 0:
                node["min_length"] = facets["minLength"]
            if "maxLength" in facets:
                node["max_length"] = facets["maxLength"]
        elif kind in ("integer", "decimal"):
            # max_length carries the maximum number of digits (totalDigits).
            if "totalDigits" in facets:
                node["max_length"] = facets["totalDigits"]
            if "fractionDigits" in facets:
                node["fraction_digits"] = facets["fractionDigits"]
        if kind in ("string", "code", "integer", "decimal") and "pattern" in facets:
            pattern = facets["pattern"]
            owner = info["pattern_type"]
            override = self.pattern_overrides.get(owner)
            if override is not None:
                require_source(override, "pattern_overrides[%s]" % owner)
                if override.get("strip_anchors"):
                    pattern = strip_anchors(pattern)
            check_regex(pattern, "%s (type %s)" % (template, owner))
            node["pattern"] = pattern
        if info.get("value_pattern"):
            if kind != "integer" or "pattern" in node:
                fail("%s: value space of %s cannot be combined with a pattern or a %s type" % (template, type_name, kind))
            node["pattern"] = info["value_pattern"]
        if kind == "code":
            self.check_code_facets(node, template)
        if doc:
            node["description"] = doc
        return node

    def check_code_facets(self, node, template):
        """Every code of the list must satisfy the length and pattern facets of the node."""
        lst = self.code_lists[node["code_list"]]
        if lst.get("unavailable"):
            return
        pattern = re.compile("(?:%s)" % node["pattern"], re.ASCII) if "pattern" in node else None
        for code in lst["codes"]:
            value = code["value"]
            bad = None
            if len(value) < node.get("min_length", 0):
                bad = "shorter than min_length %d" % node["min_length"]
            elif "max_length" in node and len(value) > node["max_length"]:
                bad = "longer than max_length %d" % node["max_length"]
            elif pattern is not None and not pattern.fullmatch(value):
                bad = "does not match pattern %s" % node["pattern"]
            if bad:
                fail("%s: code %r of list %s is %s; exclude it in constraints.json code_exclusions (with a source) or fix the list" % (template, value, node["code_list"], bad))

    def add_inline_enum(self, list_name, values):
        if list_name in self.code_lists:
            existing = [c["value"] for c in self.code_lists[list_name]["codes"]]
            if existing != values:
                fail("inline list %s defined twice with different values" % list_name)
            return
        self.code_lists[list_name] = {
            "name": list_name,
            "codes": [{"value": v, "label": v} for v in values],
            "source": "%s: enumeration of type %s" % (self.xsd_name, list_name),
        }

    def type_doc(self, info):
        return info.get("doc", "")

    def build_root(self):
        if "ds" not in self.top_elements:
            fail("root element ds not found")
        ds = self.top_elements["ds"]
        node = base_node("ds", "", "group", 1, 1)
        ctype = ds.find(XS + "complexType")
        doc = documentation(ds)
        if doc:
            node["description"] = doc
        items, attrs = self.collect(ctype)
        node["children"] = self.build_children(items, attrs, "")
        return node

    def collect(self, ct):
        """Return (particles, attributes) of a complex type with element content."""
        items = []
        attrs = []
        for child in ct:
            name = tag(child)
            if name == "annotation":
                continue
            if name in ("sequence", "choice", "all"):
                items.extend(self.walk_particle(child))
            elif name == "attribute":
                attrs.append(("", child))
            elif name == "complexContent":
                ext = child.find(XS + "extension")
                if ext is None:
                    fail("complexContent: only xs:extension is supported")
                base = ext.get("base")
                if base in self.complex_types:
                    b_items, b_attrs = self.collect(self.complex_types[base])
                    items.extend(b_items)
                    attrs.extend(b_attrs)
                elif base != "xs:anyType":
                    fail("complexContent: base %s not supported" % base)
                for sub in ext:
                    sname = tag(sub)
                    if sname in ("sequence", "choice", "all"):
                        items.extend(self.walk_particle(sub))
                    elif sname == "attribute":
                        attrs.append(("", sub))
                    elif sname != "annotation":
                        fail("complexContent: %s not supported" % sname)
            elif name == "simpleContent":
                fail("unexpected simple content here")
            else:
                fail("complexType: %s not supported" % name)
        return items, attrs

    def walk_particle(self, particle):
        kind = tag(particle)
        if kind == "all":
            fail("xs:all not supported")
        if kind == "choice":
            fail("xs:choice not supported (neither the validator nor the XML writer can pick a branch)")
        if kind == "sequence":
            if particle.get("minOccurs", "1") != "1" or particle.get("maxOccurs", "1") != "1":
                fail("xs:sequence with cardinality not supported")
            out = []
            for sub in particle:
                if tag(sub) != "annotation":
                    out.extend(self.walk_particle(sub))
            return out
        if kind == "element":
            return [("element", particle, None)]
        fail("particle %s not supported" % kind)

    def build_children(self, items, attrs, parent_template):
        children = []
        for owner, attr in attrs:
            child = self.build_attribute(owner, attr, parent_template)
            if child is not None:
                children.append(child)
        for _kind, el, _inner in items:
            child = self.build_element(el, parent_template)
            if child is not None:
                children.append(child)
        return children

    def is_excluded(self, el, template):
        name = el.get("name")
        if template in self.excluded_templates:
            self.excluded_templates[template] = True
            matched = True
        elif name in self.excluded_names:
            self.excluded_names[name] = True
            matched = True
        else:
            matched = False
        if matched and el.get("minOccurs", "1") != "0":
            fail("cannot exclude %s: the element is mandatory in the XSD" % template)
        return matched

    def build_element(self, el, parent_template):
        name = el.get("name")
        mn, mx = occurs(el)
        template = join_template(parent_template, name + ("[]" if mx < 0 or mx > 1 else ""))
        if self.is_excluded(el, template):
            return None
        own_doc = documentation(el)
        type_name = el.get("type")
        inline_complex = el.find(XS + "complexType")
        inline_simple = el.find(XS + "simpleType")

        if inline_complex is not None:
            return self.build_complex(el, inline_complex, name, template, mn, mx, own_doc, "")
        if inline_simple is not None:
            info = self.inline_simple_info(inline_simple, template)
            return self.leaf_from_info(info, name, template, mn, mx, False, own_doc or self.type_doc(info), "anonyme")
        if type_name is None:
            fail("%s: element without type" % template)
        if type_name in self.complex_types:
            ct = self.complex_types[type_name]
            if ct.find(XS + "simpleContent") is not None:
                return self.build_simple_content(name, template, mn, mx, own_doc, type_name, ct)
            return self.build_complex(el, ct, name, template, mn, mx, own_doc or documentation(ct), type_name)
        info = self.simple_info(type_name)
        return self.leaf_from_info(info, name, template, mn, mx, False, own_doc or self.type_doc(info), type_name)

    def visible_attrs(self, attrs, template):
        """Return (attributes kept as nodes, text attribute of a code_or_text element or None)."""
        out = []
        text_attr = None
        for owner, attr in attrs:
            key = (owner, attr.get("name"))
            if key in self.code_or_text:
                entry = self.code_or_text[key]
                entry["used"] = True
                if attr.get("use", "optional") == "required":
                    fail("%s: code_or_text attribute %s.%s must be optional" % (template, key[0], key[1]))
                if text_attr is not None:
                    fail("%s: several code_or_text attributes" % template)
                info = self.simple_info(attr.get("type") or "")
                if not info["enum"] or entry["value"] not in info["enum"]:
                    fail("%s: code_or_text attribute %s.%s is not an enumeration containing %r" % (template, key[0], key[1], entry["value"]))
                text_attr = {"name": key[1], "value": entry["value"]}
                continue
            if key in self.dropped:
                self.dropped[key] = True
                if attr.get("use") == "required":
                    fail("mandatory attribute %s.%s cannot be dropped" % key)
                continue
            out.append((owner, attr))
        return out, text_attr

    def mark_code_or_text(self, node, text_attr, template):
        if text_attr is None:
            return node
        if node["kind"] != "field" or node.get("type") != "code":
            fail("%s: code_or_text requires a single code field" % template)
        node["code_or_text"] = True
        node["text_attribute"] = text_attr
        return node

    def build_simple_content(self, name, template, mn, mx, own_doc, type_name, ct):
        info, attrs = self.complex_text_info(type_name)
        attrs, text_attr = self.visible_attrs(attrs, template)
        doc = own_doc or documentation(ct) or self.type_doc(info)
        if not attrs:
            leaf = self.leaf_from_info(info, name, template, mn, mx, False, doc, type_name)
            return self.mark_code_or_text(leaf, text_attr, template)
        if text_attr is not None:
            fail("%s: code_or_text element cannot carry other attributes" % template)
        return self.group_with_text(name, template, mn, mx, doc, info, attrs, type_name)

    def build_complex(self, el, ct, name, template, mn, mx, doc, type_name):
        sc = ct.find(XS + "simpleContent")
        if sc is not None:
            # anonymous complex type with simple content (e.g. numeroIdentification)
            derivation = sc.find(XS + "extension")
            if derivation is None:
                fail("%s: anonymous simpleContent without extension" % template)
            base = derivation.get("base")
            if base in self.complex_types:
                base_info, attrs = self.complex_text_info(base)
            else:
                base_info, attrs = self.simple_info(base), []
            attrs = list(attrs) + [("", a) for a in derivation.findall(XS + "attribute")]
            attrs, text_attr = self.visible_attrs(attrs, template)
            info = dict(base_info)
            if not attrs:
                leaf = self.leaf_from_info(info, name, template, mn, mx, False, doc, "anonyme")
                return self.mark_code_or_text(leaf, text_attr, template)
            if text_attr is not None:
                fail("%s: code_or_text element cannot carry other attributes" % template)
            return self.group_with_text(name, template, mn, mx, doc, info, attrs, "anonyme")
        node = base_node(name, template, "group", mn, mx)
        if doc:
            node["description"] = doc
        items, attrs = self.collect(ct)
        attrs, text_attr = self.visible_attrs(attrs, template)
        if text_attr is not None:
            fail("%s: code_or_text attribute on an element with element content" % template)
        node["children"] = self.build_children(items, attrs, template)
        if not node["children"]:
            fail("%s: group without any child after exclusions" % template)
        return node

    def group_with_text(self, name, template, mn, mx, doc, info, attrs, type_name):
        """Element with simple content and kept attributes: group of attributes + `_text`."""
        node = base_node(name, template, "group", mn, mx)
        if doc:
            node["description"] = doc
        children = []
        for owner, attr in attrs:
            child = self.build_attribute(owner, attr, template)
            if child is not None:
                children.append(child)
        text_template = join_template(template, "_text")
        # The element text is mandatory when the base type has a minimum length.
        text_min = 1 if info["facets"].get("minLength", 0) > 0 else 0
        children.append(self.leaf_from_info(info, "_text", text_template, text_min, 1, False, "", type_name))
        node["children"] = children
        return node

    def build_attribute(self, owner, attr, parent_template):
        name = attr.get("name")
        template = join_template(parent_template, name)
        use = attr.get("use", "optional")
        mn = 1 if use == "required" else 0
        doc = documentation(attr)
        type_name = attr.get("type")
        inline = attr.find(XS + "simpleType")
        if inline is not None:
            info = self.inline_simple_info(inline, template)
            type_name = "anonyme"
        elif type_name is not None:
            info = self.simple_info(type_name)
        else:
            fail("%s: attribute without type" % template)
        return self.leaf_from_info(info, name, template, mn, 1, True, doc or self.type_doc(info), type_name)


def base_node(name, template, kind, mn, mx):
    return {"name": name, "template": template, "kind": kind, "min": mn, "max": mx, "export": True}


def occurs(el):
    mn = int(el.get("minOccurs", "1"))
    raw = el.get("maxOccurs", "1")
    mx = -1 if raw == "unbounded" else int(raw)
    return mn, mx


def join_template(parent, name):
    return parent + "." + name if parent else name


def require_source(entry, where):
    if not isinstance(entry, dict) or not str(entry.get("source", "")).strip():
        fail("%s: source field is required" % where)


# --------------------------------------------------------------------------
# Post-processing driven by constraints.json
# --------------------------------------------------------------------------


def walk(node):
    yield node
    for child in node.get("children", []):
        for sub in walk(child):
            yield sub


def glob_to_regex(glob):
    out = []
    for char in glob:
        out.append("[^.\\[\\]]+" if char == "*" else re.escape(char))
    return re.compile("^" + "".join(out) + "$")


def index_by_template(root):
    index = {}
    for node in walk(root):
        if node["template"] in index and node["template"] != "":
            fail("duplicate template: %s" % node["template"])
        index[node["template"]] = node
    return index


def under(template, prefix):
    return template == prefix or template.startswith(prefix + ".")


def working_field(gen, parent_template, field):
    """Build one working (non exported) field node under parent_template."""
    ftemplate = join_template(parent_template, field["name"])
    node = base_node(field["name"], ftemplate, "field", field.get("min", 0), field.get("max", 1))
    node["export"] = False
    node["type"] = field["type"]
    if field["type"] not in FIELD_TYPES and field["type"] != "ref":
        fail("%s: unknown type %r" % (ftemplate, field["type"]))
    for key in ("max_length", "min_length", "pattern", "code_list", "ref_targets", "label", "description"):
        if key in field:
            node[key] = field[key]
    if field.get("code_or_text"):
        if field["type"] != "code":
            fail("%s: code_or_text requires a code field" % ftemplate)
        node["code_or_text"] = True
    if "pattern" in node:
        check_regex(node["pattern"], ftemplate)
    if node["type"] == "code":
        list_name = node.get("code_list")
        if list_name is None:
            fail("%s: code field without code_list" % ftemplate)
        if list_name not in gen.code_lists:
            if "+" in list_name:
                gen.merged_list(list_name.split("+"))
            else:
                fail("%s: unknown code list %s" % (ftemplate, list_name))
    return node


def add_working_nodes(gen, root):
    spec = gen.constraints["working_nodes"]
    require_source(spec, "working_nodes")
    wroot = spec["root"]
    work = base_node(wroot["name"], wroot["name"], "group", 0, 1)
    work["export"] = False
    work["label"] = wroot["label"]
    work["description"] = wroot["description"]
    items = spec["items"]
    template = join_template(work["template"], items["name"] + "[]")
    tx = base_node(items["name"], template, "group", items["min"], items["max"])
    tx["export"] = False
    tx["label"] = items["label"]
    tx["description"] = items["description"]
    tx["children"] = [working_field(gen, template, field) for field in items["fields"]]
    # Fields directly under `work` (computed totals, judgment choices), before the items.
    work["children"] = [working_field(gen, work["template"], field) for field in wroot.get("fields", [])] + [tx]
    root["children"].append(work)


def apply_post(gen, root):
    index = index_by_template(root)
    c = gen.constraints

    # excluded entries must all have matched something
    for template, used in sorted(gen.excluded_templates.items()):
        if not used:
            fail("excluded: %s excluded nothing (stale entry)" % template)
    for name, used in sorted(gen.excluded_names.items()):
        if not used:
            fail("excluded: element %s excluded nothing (stale entry)" % name)
    for key, used in sorted(gen.dropped.items()):
        if not used:
            fail("dropped_attributes: %s.%s dropped nothing (stale entry)" % key)

    for key, entry in sorted(gen.code_or_text.items()):
        if not entry["used"]:
            fail("code_or_text: %s.%s matched nothing (stale entry)" % key)

    # refs
    for entry in c["refs"]:
        require_source(entry, "refs[%s]" % entry.get("template"))
        node = index.get(entry["template"])
        if node is None:
            fail("refs: template not found %s" % entry["template"])
        for target in entry["targets"]:
            if target not in index:
                fail("refs: target not found %s" % target)
        node["ref_targets"] = list(entry["targets"])
        if entry.get("ref_or_text"):
            # Id of a declared item or free text: stays a string with its facets.
            if node.get("type") != "string":
                fail("refs: %s: ref_or_text requires a text field" % entry["template"])
            node["ref_or_text"] = True
        else:
            node["type"] = "ref"
            for key in ("max_length", "min_length", "pattern", "fraction_digits", "code_list"):
                node.pop(key, None)
    for node in walk(root):
        if node.get("type") == "ref" or node.get("ref_or_text"):
            for target in node.get("ref_targets", []):
                if target not in index:
                    fail("%s: ref target not found %s" % (node["template"], target))

    # export ids
    for entry in c["export_ids"]:
        require_source(entry, "export_ids[%s]" % entry.get("template"))
        pattern = glob_to_regex(entry["template"])
        matched = [n for n in walk(root) if pattern.match(n["template"])]
        if not matched:
            fail("export_ids: %s matches no node" % entry["template"])
        for node in matched:
            if node["kind"] != "field":
                fail("export_ids: %s is not a field" % node["template"])
            node["export_id"] = True

    # natures: the nearest entry (longest template) applies to the whole subtree
    natures = c["natures"]
    for entry in natures:
        require_source(entry, "natures[%s]" % entry.get("template"))
        if entry["template"] not in index:
            fail("natures: template not found %s" % entry["template"])
        if entry["nature"] not in NATURES:
            fail("natures: unknown nature %r" % entry["nature"])
    for node in walk(root):
        best = None
        for entry in natures:
            if node["template"] != "" and under(node["template"], entry["template"]):
                if best is None or len(entry["template"]) > len(best["template"]):
                    best = entry
        if best is not None and best["nature"] != "copied":
            node["nature"] = best["nature"]

    # labels and descriptions
    for entry in c["labels"]:
        require_source(entry, "labels[%s]" % entry.get("template"))
        node = index.get(entry["template"])
        if node is None:
            fail("labels: template not found %s" % entry["template"])
        node["label"] = entry["label"]
    for entry in c["descriptions"]:
        require_source(entry, "descriptions[%s]" % entry.get("template"))
        node = index.get(entry["template"])
        if node is None:
            fail("descriptions: template not found %s" % entry["template"])
        node["description"] = entry["text"]


def expand_rules(gen, index):
    rules = []
    seen = set()
    for entry in gen.constraints["rules"]:
        scopes = entry.get("for_each_scope")
        if scopes is None:
            variants = [(entry["id"], None)]
        else:
            variants = [(entry["id"] + "@" + scope.replace("[]", ""), scope) for scope in scopes]
        for rule_id, scope in variants:
            rule = build_rule(entry, rule_id, scope)
            if rule_id in seen:
                fail("duplicate rule: %s" % rule_id)
            seen.add(rule_id)
            check_rule(rule, index, gen.code_lists)
            rules.append(rule)
    return rules


def build_rule(entry, rule_id, scope):
    def sub(text):
        return text.replace("{scope}", scope) if scope is not None else text

    require_source(entry, "rules[%s]" % entry["id"])
    rule = {"id": rule_id, "kind": entry["kind"], "description": entry["description"], "source": entry["source"]}
    rule["scope"] = sub(entry["scope"]) if "scope" in entry else scope
    if "severity" in entry:
        rule["severity"] = entry["severity"]
    if "then" in entry:
        rule["then"] = [sub(t) for t in entry["then"]]
    if "among" in entry:
        rule["among"] = [sub(t) for t in entry["among"]]
    if "when" in entry:
        when = dict(entry["when"])
        when["path"] = sub(when["path"])
        rule["when"] = when
    return rule


def check_rule(rule, index, code_lists):
    rid = rule["id"]
    if rule["kind"] not in RULE_KINDS:
        fail("rule %s: unknown kind %r" % (rid, rule["kind"]))
    if rule.get("severity", "blocking") not in ("blocking", "warning"):
        fail("rule %s: unknown severity" % rid)
    if not rule.get("scope") or rule["scope"] not in index:
        fail("rule %s: scope not found %r" % (rid, rule.get("scope")))
    if rule["kind"] in ("required_if", "forbidden_if"):
        if not rule.get("then"):
            fail("rule %s: then is required" % rid)
        if "when" not in rule:
            fail("rule %s: when is required" % rid)
    else:
        if len(rule.get("among", [])) < 2:
            fail("rule %s: among must list at least two templates" % rid)
    paths = list(rule.get("then", [])) + list(rule.get("among", []))
    if "when" in rule:
        when = rule["when"]
        if when.get("op") not in CONDITION_OPS:
            fail("rule %s: unknown operator %r" % (rid, when.get("op")))
        if when["op"] in ("eq", "in", "not_in") and not when.get("values"):
            fail("rule %s: values required for %s" % (rid, when["op"]))
        paths.append(when["path"])
    for path in paths:
        if path not in index:
            fail("rule %s: template not found %s" % (rid, path))
        scope = rule["scope"]
        if not under(path, scope) and "[]" in path:
            fail("rule %s: %s is repeated but outside the scope %s" % (rid, path, scope))
    # code values used in conditions must exist in the bound list
    if "when" in rule and rule["when"].get("values"):
        node = index[rule["when"]["path"]]
        lst = node.get("code_list")
        if lst is not None:
            check_values(code_lists, rid, lst, rule["when"]["values"])


def check_values(code_lists, rid, list_name, values):
    lst = code_lists.get(list_name)
    if lst is None or lst.get("unavailable"):
        return
    known = {c["value"] for c in lst["codes"]}
    for value in values:
        if value not in known:
            fail("rule %s: value %r is not in list %s" % (rid, value, list_name))


def check_hand_rules(gen):
    seen = set()
    out = []
    for entry in gen.constraints["hand_rules"]:
        require_source(entry, "hand_rules[%s]" % entry.get("id"))
        if not entry.get("id") or not entry.get("description"):
            fail("hand_rules: id and description are required")
        if entry["id"] in seen:
            fail("hand_rules: duplicate id %s" % entry["id"])
        seen.add(entry["id"])
        out.append(entry["id"])
    return out


def build_defaults(gen, index):
    out = {}
    for entry in gen.constraints["defaults"]:
        require_source(entry, "defaults[%s]" % entry.get("template"))
        node = index.get(entry["template"])
        if node is None or node["kind"] != "field":
            fail("defaults: field not found %s" % entry["template"])
        out[entry["template"]] = entry["value"]
    return out


def generate(version):
    here = os.path.dirname(os.path.abspath(__file__))
    version_dir = os.path.normpath(os.path.join(here, "..", "versions", version))
    if not os.path.isdir(version_dir):
        fail("unknown version: %s (folder %s missing)" % (version, version_dir))
    gen = Generator(version, version_dir)
    key = gen.manifest["format"].split(":")
    if len(key) != 3 or key[2] != version:
        fail("manifest.format %r does not match version %s" % (gen.manifest["format"], version))

    root = gen.build_root()
    add_working_nodes(gen, root)
    apply_post(gen, root)
    index = index_by_template(root)
    rules = expand_rules(gen, index)
    hand = check_hand_rules(gen)
    defaults = build_defaults(gen, index)

    gen.apply_unknown_codes()
    gen.apply_other_codes()

    # every referenced code list must exist
    for node in walk(root):
        if node.get("code_list") and node["code_list"] not in gen.code_lists:
            fail("%s: code list missing %s" % (node["template"], node["code_list"]))

    inputs = [{"path": e["path"], "sha256": e["sha256"]} for e in gen.manifest["inputs"]]
    spec = {
        "key": {"family": key[0], "country": key[1], "version": key[2]},
        "report_types": gen.constraints["report_types"],
        "root": root,
        "code_lists": gen.code_lists,
        "rules": rules,
        "inputs": inputs,
        "defaults": defaults,
    }
    nodes = [n for n in walk(root) if n["template"] != ""]
    stats = {
        "nodes": len(nodes),
        "leaves": len([n for n in nodes if n["kind"] == "field"]),
        "code_lists": len(gen.code_lists),
        "rules": len(rules),
        "hand_rules": len(hand),
    }
    return spec, stats


def main(argv):
    parser = argparse.ArgumentParser(description="Generate the TRACFIN spec.gen.json for one format version")
    parser.add_argument("--version", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args(argv)
    if not re.match(r"^[0-9]+(\.[0-9]+)*$", args.version):
        sys.stderr.write("generate.py: invalid version %r\n" % args.version)
        return 2
    try:
        spec, stats = generate(args.version)
    except GenError as error:
        sys.stderr.write("generate.py: %s\n" % error)
        return 1
    text = json.dumps(spec, indent=1, sort_keys=True, ensure_ascii=False) + "\n"
    with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(text)
    sys.stderr.write(
        "spec %s: %d nodes, %d leaves, %d lists, %d rules, %d hand rules\n"
        % (args.version, stats["nodes"], stats["leaves"], stats["code_lists"], stats["rules"], stats["hand_rules"])
    )
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
