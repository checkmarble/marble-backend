package pure_utils

import "sort"

func SlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sortedA := append([]string(nil), a...)
	sortedB := append([]string(nil), b...)
	sort.Strings(sortedA)
	sort.Strings(sortedB)
	for i, dataset := range sortedA {
		if dataset != sortedB[i] {
			return false
		}
	}
	return true
}
