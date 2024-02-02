package models

import (
	"fmt"
	"slices"

	"github.com/hashicorp/go-set/v2"
)

type IndexFamily struct {
	Fixed  []FieldName
	Flex   *set.Set[FieldName]
	Last   FieldName
	Others *set.Set[FieldName]
}

func NewIndexFamily() IndexFamily {
	return IndexFamily{
		Fixed:  make([]FieldName, 0),
		Flex:   set.New[FieldName](0),
		Last:   "",
		Others: set.New[FieldName](0),
	}
}

func (f IndexFamily) Equal(other IndexFamily) bool {
	return slices.Equal(f.Fixed, other.Fixed) &&
		f.Flex.Equal(other.Flex) &&
		(f.Last == other.Last) &&
		f.Others.Equal(other.Others)
}

func (f IndexFamily) Hash() string {
	fl := ""
	if f.Flex != nil {
		s := f.Flex.Slice()
		slices.Sort(s)
		fl = fmt.Sprintf("%v", s)
	}
	ot := ""
	if f.Others != nil {
		s := f.Others.Slice()
		slices.Sort(s)
		ot = fmt.Sprintf("%v", s)
	}
	return fmt.Sprintf("%v %s %s %s", f.Fixed, fl, f.Last, ot)
}

func (f IndexFamily) copy() IndexFamily {
	return IndexFamily{
		Fixed:  slices.Clone(f.Fixed),
		Flex:   f.Flex.Copy(),
		Last:   f.Last,
		Others: f.Others.Copy(),
	}
}

func (f IndexFamily) allIndexedValues() *set.Set[FieldName] {
	out := f.Flex.Union(set.From(f.Fixed)).(*set.Set[FieldName])
	if f.Last != "" {
		out.Insert(f.Last)
	}
	return out
}

func (f IndexFamily) size() int {
	s := len(f.Fixed) + f.Flex.Size()
	if f.Last != "" {
		s++
	}
	return s
}

func (f IndexFamily) removeFixedPrefix(prefix []FieldName) IndexFamily {
	if len(prefix) > len(f.Fixed) {
		return IndexFamily{}
	}
	if !slices.Equal(f.Fixed[:len(prefix)], prefix) {
		return IndexFamily{}
	}
	return IndexFamily{
		Fixed:  f.Fixed[len(prefix):],
		Flex:   f.Flex.Copy(),
		Last:   f.Last,
		Others: f.Others.Copy(),
	}
}

func (f IndexFamily) prependPrefix(prefix []FieldName) IndexFamily {
	return IndexFamily{
		Fixed:  append(prefix, f.Fixed...),
		Flex:   f.Flex.Copy(),
		Last:   f.Last,
		Others: f.Others.Copy(),
	}
}

func (f *IndexFamily) setLast(last FieldName) {
	if last == "" {
		f.Last = ""
		return
	}

	if f.Flex.Empty() {
		f.Last = ""
		f.Fixed = append(f.Fixed, last)
	} else {
		f.Last = last
	}
}

func ExtractMinimalSetOfIdxFamilies(idxFamiliesIn *set.HashSet[IndexFamily, string]) *set.HashSet[IndexFamily, string] {
	// We iterate over the input set of families, and try to reduce the number in the ouput step by step by combining families
	// or indexes where possible
	output := []IndexFamily{}
	input := idxFamiliesIn.Slice()
	slices.SortFunc(input, compareIdxFamily)

	for _, idxIn := range input {
		foundReplacement := false
		for _, idxOut := range output {
			combined, ok := refineIdxFamilies(idxOut, idxIn)
			if ok {
				output = append(output, combined)
				foundReplacement = true
				break
			}
		}
		if !foundReplacement {
			output = append(output, idxIn)
		}
	}

	return set.HashSetFrom(output)
}

func compareIdxFamily(a, b IndexFamily) int {
	if a.Hash() < b.Hash() {
		return -1
	} else if a.Hash() == b.Hash() {
		return 0
	}
	return 1
}

func refineIdxFamilies(left, right IndexFamily) (IndexFamily, bool) {
	// first, we try to go back to the case where one at least of the Fixed values
	// (AKA the values where we already know the order in which the output has to be)
	// is empty, by removing a common prefix from the Fixed values if necessary
	prefixLen := min(len(left.Fixed), len(right.Fixed))
	if prefixLen > 0 {
		for i := 0; i < prefixLen; i++ {
			if i < len(left.Fixed) && i < len(right.Fixed) && left.Fixed[i] != right.Fixed[i] {
				return IndexFamily{}, false
			}
		}
		fixedPrefix := left.Fixed[:prefixLen]
		out, ok := refineIdxFamilies(
			left.removeFixedPrefix(fixedPrefix),
			right.removeFixedPrefix(fixedPrefix),
		)
		out = out.prependPrefix(fixedPrefix)
		return out, ok
	}

	// Now we know that one of the families at least has an empty fixed values slice
	var short, long IndexFamily
	// if len(left.Fixed) < len(right.Fixed) {
	if len(right.Fixed) == 0 && len(left.Fixed) > 0 {
		short = right
		long = left
	} else {
		short = left
		long = right
	}

	return refineIdxFamiliesShortHasNoFixed(short, long)
}

func (f IndexFamily) mergeOthers(B IndexFamily) IndexFamily {
	out := f.copy()
	out.Others = out.Others.Union(B.Others).(*set.Set[FieldName])
	out.Others = out.Others.Difference(out.allIndexedValues()).(*set.Set[FieldName])
	return out
}

func refineIdxFamiliesShortHasNoFixed(A, B IndexFamily) (IndexFamily, bool) {
	// we know by hypothesis that A.Fixed = []
	if A.size() > B.size() {
		// If some values in B are not in A, it's easy to see that there is no solution (A.Last comes after so can't be used)
		if !A.allIndexedValues().Subset(B.Flex) {
			return IndexFamily{}, false
		}

		// We know that B's values are included in A.Flex
		out := B.copy()
		if B.Last != "" {
			// Now we know that B.Last != "" so we'll need to make some choices
			fixAppend := B.Flex.Slice()
			slices.Sort(fixAppend)
			// here we make an arbitrary choice (but deterministic) on the order of the fields
			out.Fixed = append(out.Fixed, fixAppend...)
			out.Fixed = append(out.Fixed, B.Last)
		}
		out.Flex = A.Flex.Difference(set.From(out.Fixed)).(*set.Set[FieldName])
		out.setLast(A.Last)
		out = out.mergeOthers(A)
		return out, true
	}

	if A.size() == B.size() {
		// if they have the same size, it must be that the same columns are indexed (even if in different order)
		if !A.allIndexedValues().Equal(B.allIndexedValues()) {
			return IndexFamily{}, false
		}

		// Now in the following, we know that the same columns are indexed

		if B.Last != "" {
			if A.Last != "" && A.Last != B.Last {
				return IndexFamily{}, false
			}
			out := B.copy()
			out = out.mergeOthers(A)
			return out, true
		}

		// So B.Last == ""
		if A.Last == "" {
			out := B.copy()
			out = out.mergeOthers(A)
			return out, true
		}

		// So B.Last=="" and A.Last != ""
		if B.Flex.Empty() {
			if A.Last != B.Fixed[len(B.Fixed)-1] {
				return IndexFamily{}, false
			}
			out := B.copy()
			out = out.mergeOthers(A)
			return out, true
		}

		// Last case: B.Last=="" and B.Flex not empty and A.Last != ""
		if !B.Flex.Contains(A.Last) {
			return IndexFamily{}, false
		}
		out := B.copy()
		out.Flex.Remove(A.Last)
		out.setLast(A.Last)
		out = out.mergeOthers(A)
		return out, true
	}

	// So A.Size() < B.Size()
	if A.size() <= len(B.Fixed) {
		if A.Last == "" {
			if A.Flex.Equal(set.From(B.Fixed[:A.size()])) {
				out := B.copy()
				out.Flex = B.Flex.Difference(set.From(B.Fixed[:A.size()])).(*set.Set[FieldName])
				out = out.mergeOthers(A)
				return out, true
			}
			return IndexFamily{}, false
		}
		// A.Last != ""
		if B.Fixed[A.size()-1] != A.Last {
			return IndexFamily{}, false
		}
		out := B.copy()
		out = out.mergeOthers(A)
		return out, true
	}
	// so A.Size() > len(B.Fixed)
	if !A.Flex.Subset(set.From(B.Fixed)) {
		return IndexFamily{}, false
	}
	// So B.Fixed is included in A.Flex
	if A.Last == "" {
		out := B.copy()
		out = out.mergeOthers(A)
		return out, true
	}
	// So A.Last != ""
	if !B.Flex.Contains(A.Last) {
		return IndexFamily{}, false
	}
	out := B.copy()
	appendToFix := B.Flex.Intersect(A.Flex).(*set.Set[FieldName]).Slice()
	// arbitrary order here
	slices.Sort(appendToFix)
	out.Fixed = append(out.Fixed, appendToFix...)
	out.Fixed = append(out.Fixed, A.Last)
	out.Flex = B.Flex.Difference(set.From(out.Fixed)).(*set.Set[FieldName])
	out = out.mergeOthers(A)
	return out, true
}
