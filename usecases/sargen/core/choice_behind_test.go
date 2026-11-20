package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func TestChoiceBehind(t *testing.T) {
	tx, acc, holder, loose := core.ObjectRef{Table: "tx", ObjectID: "1"}, core.ObjectRef{Table: "acc", ObjectID: "1"},
		core.ObjectRef{Table: "p", ObjectID: "1"}, core.ObjectRef{Table: "p", ObjectID: "2"}
	sc := core.StructuredContext{
		Selection:   []core.ObjectRef{tx, acc, holder, loose},
		ContextOnly: map[core.ObjectRef]bool{acc: true, holder: true, loose: true},
		ReachedFrom: map[core.ObjectRef]core.ObjectRef{acc: tx, holder: acc, loose: {Table: "gone", ObjectID: "9"}},
	}
	got, depth, ok := sc.ChoiceBehind(holder)
	assert.True(t, ok, "followed through the auto rows up to the analyst object")
	assert.Equal(t, tx, got)
	assert.Equal(t, 2, depth)
	got, depth, ok = sc.ChoiceBehind(acc)
	assert.True(t, ok)
	assert.Equal(t, tx, got)
	assert.Equal(t, 1, depth)

	_, _, ok = sc.ChoiceBehind(tx)
	assert.False(t, ok, "an analyst object has no chain")
	_, _, ok = sc.ChoiceBehind(loose)
	assert.False(t, ok, "the chain stops on an object that is not selected")

	sc.ReachedFrom[acc] = holder // loop
	_, _, ok = sc.ChoiceBehind(holder)
	assert.False(t, ok, "a loop ends without a choice")

	_, _, ok = core.StructuredContext{}.ChoiceBehind(acc)
	assert.False(t, ok)
}
