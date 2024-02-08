package models

import (
	"fmt"
	"slices"

	"github.com/hashicorp/go-set/v2"
)

type IndexFamily struct {
	TableName TableName
	Fixed     []FieldName
	Flex      *set.Set[FieldName]
	Last      FieldName
	Included  *set.Set[FieldName]
}

func NewIndexFamily() IndexFamily {
	return IndexFamily{
		Fixed:    make([]FieldName, 0),
		Flex:     set.New[FieldName](0),
		Last:     "",
		Included: set.New[FieldName](0),
	}
}

func (f IndexFamily) Equal(other IndexFamily) bool {
	return slices.Equal(f.Fixed, other.Fixed) &&
		f.Flex.Equal(other.Flex) &&
		(f.Last == other.Last) &&
		f.Included.Equal(other.Included)
}

func (f IndexFamily) Hash() string {
	fl := ""
	if f.Flex != nil {
		s := f.Flex.Slice()
		slices.Sort(s)
		fl = fmt.Sprintf("%v", s)
	}
	ot := ""
	if f.Included != nil {
		s := f.Included.Slice()
		slices.Sort(s)
		ot = fmt.Sprintf("%v", s)
	}
	return fmt.Sprintf("%s - %v - %s - %s - %s", f.TableName, f.Fixed, fl, f.Last, ot)
}

func (f IndexFamily) copy() IndexFamily {
	return IndexFamily{
		TableName: f.TableName,
		Fixed:     slices.Clone(f.Fixed),
		Flex:      f.Flex.Copy(),
		Last:      f.Last,
		Included:  f.Included.Copy(),
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
		Fixed:    f.Fixed[len(prefix):],
		Flex:     f.Flex.Copy(),
		Last:     f.Last,
		Included: f.Included.Copy(),
	}
}

func (f IndexFamily) prependPrefix(prefix []FieldName) IndexFamily {
	return IndexFamily{
		Fixed:    append(prefix, f.Fixed...),
		Flex:     f.Flex.Copy(),
		Last:     f.Last,
		Included: f.Included.Copy(),
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

func extractMinimalSetOfIdxFamilies(idxFamiliesIn *set.HashSet[IndexFamily, string]) *set.HashSet[IndexFamily, string] {
	// We do the procedure once for every table, on every index required on that table
	output := []IndexFamily{}
	familiesByTable := groupIdxFamiliesByTable(idxFamiliesIn)
	for _, families := range familiesByTable {
		output = append(output, extractMinimalSetOfIdxFamiliesOneTable(families).Slice()...)
	}

	return set.HashSetFrom(output)
}

func extractMinimalSetOfIdxFamiliesOneTable(idxFamiliesIn *set.HashSet[IndexFamily, string]) *set.HashSet[IndexFamily, string] {
	// We iterate over the input set of families, and try to reduce the number in the ouput step by step by combining families
	// or indexes where possible
	output := []IndexFamily{}
	input := idxFamiliesIn.Slice()
	slices.SortFunc(input, compareIdxFamily)

	for _, idxIn := range input {
		foundReplacement := false
		var combined IndexFamily
		var matchIdx int
		for i, idxOut := range output {
			var ok bool
			combined, ok = refineIdxFamilies(idxOut, idxIn)
			if ok {
				output = append(output, combined)
				foundReplacement = true
				matchIdx = i
				break
			}
		}
		if foundReplacement {
			output = slices.Delete(output, matchIdx, matchIdx+1)
			output = append(output, combined)
		} else {
			output = append(output, idxIn)
		}
	}

	return set.HashSetFrom(output)
}

func groupIdxFamiliesByTable(idxFamilies *set.HashSet[IndexFamily, string]) map[TableName]*set.HashSet[IndexFamily, string] {
	out := make(map[TableName]*set.HashSet[IndexFamily, string])
	for _, idxFamily := range idxFamilies.Slice() {
		if _, ok := out[idxFamily.TableName]; !ok {
			out[idxFamily.TableName] = set.NewHashSet[IndexFamily, string](0)
		}
		out[idxFamily.TableName].Insert(idxFamily)
	}
	return out
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
		// idx L: {[x1, x2, x3, x4] {...}}
		// idx R: {[x1, x3, x4, x5] {...}}
		//              ^^
		// mismatch: no possibility to have a common concrete index for both families
		for i := 0; i < prefixLen; i++ {
			if i < len(left.Fixed) && i < len(right.Fixed) && left.Fixed[i] != right.Fixed[i] {
				return IndexFamily{}, false
			}
		}
		// idx L: {[x1, x2, x3, x4] {x5, x6...}} => {[x3, x4] {x5, x6...}}
		// idx R: {[x1, x2]         {x5...}    } => {[]       {x5...}    }
		// this allows the following algo to be simpler without loss of generality
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

	return refineIdxFamiliesFirstHasNoFixed(short, long)
}

func (f IndexFamily) mergeIncluded(B IndexFamily) IndexFamily {
	out := f.copy()
	out.Included = out.Included.Union(B.Included).(*set.Set[FieldName])
	out.Included = out.Included.Difference(out.allIndexedValues()).(*set.Set[FieldName])
	return out
}

func refineIdxFamiliesFirstHasNoFixed(A, B IndexFamily) (IndexFamily, bool) {
	// we know by hypothesis that A.Fixed = []
	if A.size() > B.size() {
		// A: {[] {x2, x3, x4, x5, x6, x7, x8} L_A? }
		// B: {[] {x1, x2, x3, x4, x5} L_B?         }
		//         ^^ missing x1 in A
		// If some values in B are not in A, it's easy to see that there is no solution (A.Last comes after so can't be used)
		if !A.allIndexedValues().Subset(B.allIndexedValues()) {
			return IndexFamily{}, false
		}

		// We know that B's values are included in A.Flex
		out := B.copy()
		if B.Last != "" {
			// A: {[], {x2,   x3, x4, x5, x6, x7, x8} L_A }
			// B: {[x1, x2], {x3, x4, x5} L_B             } with L_B=L_A
			//                            ^^          ^^ same value L_A must be in different positions in A and B, no solution
			if B.Last == A.Last {
				return IndexFamily{}, false
			}
			// Now we know that B.Last != "" so we'll need to make some choices
			fixAppend := B.Flex.Slice()
			slices.Sort(fixAppend)
			// here we make an arbitrary choice (but deterministic) on the order of the fields
			out.Fixed = append(out.Fixed, fixAppend...)
			out.Fixed = append(out.Fixed, B.Last)
		}
		out.Flex = A.Flex.Difference(set.From(out.Fixed)).(*set.Set[FieldName])
		out.setLast(A.Last)
		out = out.mergeIncluded(A)
		return out, true
	}

	if A.size() == B.size() {
		// if they have the same size, it must be that the same columns are indexed (even if in different order)
		// A: {[] {x1, x2, x3} x4 }
		// B: {[] {x1, x2, x4} x5 }
		// 	             ^^  ^^ no x4 in B and x3 in A: no solution
		if !A.allIndexedValues().Equal(B.allIndexedValues()) {
			return IndexFamily{}, false
		}

		// Now in the following, we know that the same columns are indexed
		if B.Last != "" {
			// A: {[], {x1, x2, x3} x4 }
			// B: {[], {x1, x2, x3} x5 }
			//                      ^^ x4!=x5, no solution
			if A.Last != "" && A.Last != B.Last {
				return IndexFamily{}, false
			}
			out := B.copy()
			out = out.mergeIncluded(A)
			return out, true
		}

		// So B.Last == ""
		if A.Last == "" {
			// A: {[], {x1, x2, x3} }
			// B: {[x1, x2], {x3}   }
			// trivially keep B which is more restrictive (adding included columns)
			out := B.copy()
			out = out.mergeIncluded(A)
			return out, true
		}

		// So B.Last=="" and A.Last != ""
		if B.Flex.Empty() {
			// A: {[], {x1, x2 } x4         }
			// B: {[    x1, x2,  x3], {}    }
			//                   ^^ x3!=x4, no solution
			if A.Last != B.Fixed[len(B.Fixed)-1] {
				return IndexFamily{}, false
			}
			out := B.copy()
			out = out.mergeIncluded(A)
			return out, true
		}

		// Last case: B.Last=="" and B.Flex not empty and A.Last != ""
		if !B.Flex.Contains(A.Last) {
			// A: {[], {x1, x2,   x3 }  x5 }
			// B: {[    x1, x2]  {x3,   x4}   }
			//                    ^^^^^^^^ x5 is not in B.Flex, no solution
			return IndexFamily{}, false
		}
		out := B.copy()
		out.Flex.Remove(A.Last)
		out.setLast(A.Last)
		out = out.mergeIncluded(A)
		return out, true
	}

	// So A.Size() < B.Size()
	if A.size() <= len(B.Fixed) {
		if A.Last == "" {
			// ?? To check TODO
			if A.Flex.Equal(set.From(B.Fixed[:A.size()])) {
				out := B.copy()
				out.Flex = B.Flex.Difference(set.From(B.Fixed[:A.size()])).(*set.Set[FieldName])
				out = out.mergeIncluded(A)
				return out, true
			}
			// A: {[], {x1, x4}        }
			// B: {[    x1, x2, x3], {}}
			//              ^^ x4 is not in the 2 first values of B.Fixed: no solution
			return IndexFamily{}, false
		}
		// A.Last != ""
		// A: {[], {x1, x2} x4 }
		// B: {[    x1, x2, x3], {}}
		//                  ^^ x4 is not in the 3rd value of B.Fixed: no solution
		if B.Fixed[A.size()-1] != A.Last {
			return IndexFamily{}, false
		}
		out := B.copy()
		out = out.mergeIncluded(A)
		return out, true
	}
	// so A.Size() > len(B.Fixed)
	// A: {[], {x1, x2,  x3, x4} }
	// B: {    [x4, x5] {x1, x2, x3} }
	//              ^^ x5 is not in A.Flex: no solution
	if !A.Flex.Subset(set.From(B.Fixed)) {
		return IndexFamily{}, false
	}
	// So B.Fixed is included in A.Flex
	// A: {[], {x1,  x2, x3, x4} }
	// B: {    [x1] {x2, x3, x4, x5} }
	if A.Last == "" {
		out := B.copy()
		out = out.mergeIncluded(A)
		return out, true
	}
	// So A.Last != ""
	// A: {[], {x1,  x2, x3} x5 }
	// B: {    [x1] {x2, x3, x4} x6}
	//               ^--------^ x5 is not in B.Flex: no solution
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
	out = out.mergeIncluded(A)
	return out, true
}
