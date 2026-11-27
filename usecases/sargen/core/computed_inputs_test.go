package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type inputsFormat struct {
	Format
	inputs map[Path][]Path
}

func (f inputsFormat) ComputedInputFields(_ *Document, p Path) []Path { return f.inputs[p] }

func TestResolveComputedPaths(t *testing.T) {
	f := inputsFormat{inputs: map[Path][]Path{
		"a.computed": {"w.t1.x", "w.t2.x"},
		"b.computed": {"w.t1.x"},
	}}
	got := ResolveComputedPaths(f, &Document{}, []Path{"keep.one", "a.computed", "b.computed", "unknown.computed", "keep.one"})
	assert.Equal(t, []Path{"keep.one", "w.t1.x", "w.t2.x", "unknown.computed"}, got)
}

func TestResolveComputedPaths_NoCapability(t *testing.T) {
	got := ResolveComputedPaths(nil, &Document{}, []Path{"a", "b", "a"})
	assert.Equal(t, []Path{"a", "b"}, got)
}
