package core

import (
	"github.com/cockroachdb/errors"
	"regexp"
	"strings"
)

// Path addresses a value in a declaration document. Segments are separated by
// '.', a segment is `name` or `name[key]` where key is a stable item key.
type Path string

var (
	segmentRe     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\[([A-Za-z0-9_-]+|\+[a-z0-9]+)\])?$`)
	keyRe         = regexp.MustCompile(`\[[^\]]*\]`)
	provisionalRe = regexp.MustCompile(`\[\+[a-z0-9]+\]`)
)

// ParsePath validates s and returns it as a Path. Provisional keys (`+x`) are accepted.
func ParsePath(s string) (Path, error) {
	if s == "" {
		return "", errors.New("chemin vide")
	}
	for _, seg := range strings.Split(s, ".") {
		if !segmentRe.MatchString(seg) {
			return "", errors.Newf("segment de chemin invalide %q dans %q", seg, s)
		}
	}
	return Path(s), nil
}

// HasProvisionalKey reports whether the path contains a provisional `[+x]` key.
func (p Path) HasProvisionalKey() bool {
	return provisionalRe.MatchString(string(p))
}

// Template replaces every `[key]` by `[]`.
func (p Path) Template() string {
	return keyRe.ReplaceAllString(string(p), "[]")
}

// HasPrefix reports whether prefix is p or an ancestor of p, by segments.
func (p Path) HasPrefix(prefix Path) bool {
	if prefix == "" || p == prefix {
		return true
	}
	return strings.HasPrefix(string(p), string(prefix)+".")
}

// Parent drops the last segment.
func (p Path) Parent() Path {
	i := strings.LastIndex(string(p), ".")
	if i < 0 {
		return ""
	}
	return p[:i]
}

// Child appends a plain segment.
func (p Path) Child(name string) Path {
	if p == "" {
		return Path(name)
	}
	return Path(string(p) + "." + name)
}

// Item appends a keyed segment `name[key]`.
func (p Path) Item(name, key string) Path {
	return p.Child(name + "[" + key + "]")
}
