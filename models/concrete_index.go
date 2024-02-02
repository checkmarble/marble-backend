package models

import (
	"slices"

	"github.com/hashicorp/go-set/v2"
)

type ConcreteIndex struct {
	Indexed  []FieldName
	Included []FieldName
}

func (i ConcreteIndex) Equal(other ConcreteIndex) bool {
	return slices.Equal(i.Indexed, other.Indexed) &&
		slices.Equal(i.Included, other.Included)
}

func (i ConcreteIndex) isInstanceof(f IndexFamily) bool {
	if len(i.Indexed) < len(f.Fixed) {
		return false
	}

	if f.Last != "" {
		if !set.From(i.Indexed[len(f.Fixed):]).Equal(f.Flex) {
			return false
		}
	} else {
		if !set.From(i.Indexed[len(f.Fixed) : len(i.Indexed)-1]).Equal(f.Flex) {
			return false
		}
	}

	if f.Last != "" && f.Last != i.Indexed[len(i.Indexed)-1] {
		return false
	}

	return true
}

func selectIdxFamiliesToCreate(idxFamilies set.Collection[IndexFamily], existing []ConcreteIndex) set.Collection[IndexFamily] {
	toCreate := set.NewHashSet[IndexFamily](0)

	for _, family := range idxFamilies.Slice() {
		found := false
		for _, concreteIndex := range existing {
			if concreteIndex.isInstanceof(family) {
				found = true
				break
			}
		}
		if !found {
			toCreate.Insert(family.copy())
		}
	}

	return toCreate
}

func SelectConcreteIndexesToCreate(idxFamilies set.Collection[IndexFamily], existing []ConcreteIndex) []ConcreteIndex {
	toCreateFamilies := selectIdxFamiliesToCreate(idxFamilies, existing)

	toCreateIdx := make([]ConcreteIndex, 0, toCreateFamilies.Size())
	toCreateFamilies.ForEach(func(family IndexFamily) bool {
		indexed := family.Fixed
		indexed = append(indexed, family.Flex.Slice()...)
		if family.Last != "" {
			indexed = append(indexed, family.Last)
		}
		toCreateIdx = append(toCreateIdx, ConcreteIndex{
			Indexed:  indexed,
			Included: family.Others.Slice(),
		})
		return true
	})

	return toCreateIdx
}
