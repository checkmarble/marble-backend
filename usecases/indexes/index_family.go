package indexes

import (
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/hashicorp/go-set/v2"
)

func extractMinimalSetOfIdxFamilies(idxFamiliesIn *set.HashSet[models.IndexFamily, string]) set.Collection[models.IndexFamily] {
	// We do the procedure once for every table, on every index required on that table
	output := []models.IndexFamily{}
	familiesByTable := groupIdxFamiliesByTable(idxFamiliesIn)
	for _, families := range familiesByTable {
		output = append(output, extractMinimalSetOfIdxFamiliesOneTable(families).Slice()...)
	}

	return set.HashSetFrom(output)
}

func extractMinimalSetOfIdxFamiliesOneTable(idxFamiliesIn set.Collection[models.IndexFamily]) set.Collection[models.IndexFamily] {
	// We iterate over the input set of families, and try to reduce the number in the ouput step by step by combining families
	// or indexes where possible
	output := []models.IndexFamily{}
	input := idxFamiliesIn.Slice()
	slices.SortFunc(input, compareIdxFamily)

	for _, idxIn := range input {
		foundReplacement := false
		var combined models.IndexFamily
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

func groupIdxFamiliesByTable(idxFamilies *set.HashSet[models.IndexFamily, string]) map[models.TableName]set.Collection[models.IndexFamily] {
	out := make(map[models.TableName]set.Collection[models.IndexFamily])
	for _, idxFamily := range idxFamilies.Slice() {
		if _, ok := out[idxFamily.TableName]; !ok {
			out[idxFamily.TableName] = set.NewHashSet[models.IndexFamily, string](0)
		}
		out[idxFamily.TableName].Insert(idxFamily)
	}
	return out
}

func compareIdxFamily(a, b models.IndexFamily) int {
	if a.Hash() < b.Hash() {
		return -1
	} else if a.Hash() == b.Hash() {
		return 0
	}
	return 1
}

func refineIdxFamilies(left, right models.IndexFamily) (models.IndexFamily, bool) {
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
				return models.IndexFamily{}, false
			}
		}
		// idx L: {[x1, x2, x3, x4] {x5, x6...}} => {[x3, x4] {x5, x6...}}
		// idx R: {[x1, x2]         {x5...}    } => {[]       {x5...}    }
		// this allows the following algo to be simpler without loss of generality
		fixedPrefix := left.Fixed[:prefixLen]
		out, ok := refineIdxFamilies(
			left.RemoveFixedPrefix(fixedPrefix),
			right.RemoveFixedPrefix(fixedPrefix),
		)
		out = out.PrependPrefix(fixedPrefix)
		return out, ok
	}

	// Now we know that one of the families at least has an empty fixed values slice
	var short, long models.IndexFamily
	// if len(left.Fixed) < len(right.Fixed) {
	if len(right.Fixed) == 0 {
		short = right
		long = left
	} else {
		short = left
		long = right
	}

	return refineIdxFamiliesFirstHasNoFixed(short, long)
}

func refineIdxFamiliesFirstHasNoFixed(A, B models.IndexFamily) (models.IndexFamily, bool) {
	// we know by hypothesis that A.Fixed = []
	if A.Size() > B.Size() {
		// A: {[] {x2, x3, x4, x5, x6, x7, x8} L_A? }
		// B: {[] {x1, x2, x3, x4, x5} L_B?         }
		//         ^^ missing x1 in A
		// If some values in B are not in A, it's easy to see that there is no solution (A.Last comes after so can't be used)
		if !A.AllIndexedValues().Subset(B.AllIndexedValues()) {
			return models.IndexFamily{}, false
		}

		// We know that B's values are included in A.Flex
		out := B.Copy()
		if B.Last != "" {
			// A: {[], {x2,   x3, x4, x5, x6, x7, x8} L_A }
			// B: {[x1, x2], {x3, x4, x5} L_B             } with L_B=L_A
			//                            ^^          ^^ same value L_A must be in different positions in A and B, no solution
			if B.Last == A.Last {
				return models.IndexFamily{}, false
			}
			// Now we know that B.Last != "" so we'll need to make some choices
			fixAppend := B.Flex.Slice()
			slices.Sort(fixAppend)
			// here we make an arbitrary choice (but deterministic) on the order of the fields
			out.Fixed = append(out.Fixed, fixAppend...)
			out.Fixed = append(out.Fixed, B.Last)
		}
		out.Flex = A.Flex.Difference(set.From(out.Fixed)).(*set.Set[models.FieldName])
		out.SetLast(A.Last)
		out = out.MergeIncluded(A)
		return out, true
	}

	if A.Size() == B.Size() {
		// if they have the same Size, it must be that the same columns are indexed (even if in different order)
		// A: {[] {x1, x2, x3} x4 }
		// B: {[] {x1, x2, x4} x5 }
		// 	             ^^  ^^ no x4 in B and x3 in A: no solution
		if !A.AllIndexedValues().EqualSet(B.AllIndexedValues()) {
			return models.IndexFamily{}, false
		}

		// Now in the following, we know that the same columns are indexed
		if B.Last != "" {
			// A: {[], {x1, x2, x3} x4 }
			// B: {[], {x1, x2, x3} x5 }
			//                      ^^ x4!=x5, no solution
			if A.Last != "" && A.Last != B.Last {
				return models.IndexFamily{}, false
			}
			out := B.Copy()
			out = out.MergeIncluded(A)
			return out, true
		}

		// So B.Last == ""
		if A.Last == "" {
			// A: {[], {x1, x2, x3} }
			// B: {[x1, x2], {x3}   }
			// trivially keep B which is more restrictive (adding included columns)
			out := B.Copy()
			out = out.MergeIncluded(A)
			return out, true
		}

		// So B.Last=="" and A.Last != ""
		if B.Flex.Empty() {
			// A: {[], {x1, x2 } x4         }
			// B: {[    x1, x2,  x3], {}    }
			//                   ^^ x3!=x4, no solution
			if A.Last != B.Fixed[len(B.Fixed)-1] {
				return models.IndexFamily{}, false
			}
			out := B.Copy()
			out = out.MergeIncluded(A)
			return out, true
		}

		// Last case: B.Last=="" and B.Flex not empty and A.Last != ""
		if !B.Flex.Contains(A.Last) {
			// A: {[], {x1, x2,   x3 }  x5 }
			// B: {[    x1, x2]  {x3,   x4}   }
			//                    ^^^^^^^^ x5 is not in B.Flex, no solution
			return models.IndexFamily{}, false
		}
		out := B.Copy()
		out.Flex.Remove(A.Last)
		out.SetLast(A.Last)
		out = out.MergeIncluded(A)
		return out, true
	}

	// So we know that A.Size() < B.Size()
	// A: {[] {x1, x2, x3, x4, x5} L_A?         }
	// B: {[] {x2, x3, x4, x5, x6, x7, x8} L_B? }
	//         ^^ missing x1 in B
	// If some values in A are not in B, it's easy to see that there is no solution (B.Last comes after so can't be used)
	if !B.AllIndexedValues().Subset(A.AllIndexedValues()) {
		return models.IndexFamily{}, false
	}

	if A.Size() <= len(B.Fixed) {
		if A.Last == "" {
			// ?? To check TODO
			if A.Flex.Equal(set.From(B.Fixed[:A.Size()])) {
				out := B.Copy()
				out.Flex = B.Flex.Difference(set.From(B.Fixed[:A.Size()])).(*set.Set[models.FieldName])
				out = out.MergeIncluded(A)
				return out, true
			}
			// A: {[], {x1, x4}        }
			// B: {[    x1, x2, x3], {}}
			//              ^^ x4 is not in the 2 first values of B.Fixed: no solution
			return models.IndexFamily{}, false
		}
		// A.Last != ""
		// A: {[], {x1, x2} x4 }
		// B: {[    x1, x2, x3], {}}
		//                  ^^ x4 is not in the 3rd value of B.Fixed: no solution
		if B.Fixed[A.Size()-1] != A.Last {
			return models.IndexFamily{}, false
		}
		out := B.Copy()
		out = out.MergeIncluded(A)
		return out, true
	}
	// so A.Size() > len(B.Fixed)
	// A: {[], {x1, x2,  x3, x4} }
	// B: {    [x4, x5] {x1, x2, x3} }
	//              ^^ x5 is not in A.Flex: no solution
	if !A.Flex.Subset(set.From(B.Fixed)) {
		return models.IndexFamily{}, false
	}
	// So B.Fixed is included in A.Flex
	// A: {[], {x1,  x2, x3, x4} }
	// B: {    [x1] {x2, x3, x4, x5} }
	if A.Last == "" {
		out := B.Copy()
		out = out.MergeIncluded(A)
		return out, true
	}
	// So A.Last != ""
	// A: {[], {x1,  x2, x3} x5 }
	// B: {    [x1] {x2, x3, x4} x6}
	//               ^--------^ x5 is not in B.Flex: no solution
	if !B.Flex.Contains(A.Last) {
		return models.IndexFamily{}, false
	}
	out := B.Copy()
	appendToFix := B.Flex.Intersect(A.Flex).(*set.Set[models.FieldName]).Slice()
	// arbitrary order here
	slices.Sort(appendToFix)
	out.Fixed = append(out.Fixed, appendToFix...)
	out.Fixed = append(out.Fixed, A.Last)
	out.Flex = B.Flex.Difference(set.From(out.Fixed)).(*set.Set[models.FieldName])
	if out.Flex.Empty() {
		out.SetLast(B.Last)
	}
	out = out.MergeIncluded(A)
	return out, true
}
