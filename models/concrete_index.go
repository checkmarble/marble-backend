package models

import (
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/hashicorp/go-set/v2"
)

type ConcreteIndex struct {
	TableName TableName
	Indexed   []FieldName
	Included  []FieldName
}

func (i ConcreteIndex) Equal(other ConcreteIndex) bool {
	return i.TableName == other.TableName &&
		slices.Equal(i.Indexed, other.Indexed) &&
		slices.Equal(i.Included, other.Included)
}

func (i ConcreteIndex) covers(f IndexFamily) bool {
	// We need to make a copy of f because we are going to modify it
	// put all the identifiers to upper case, because postgres is not case sensitive as far as identifiers are concerned (unless quoted)
	// and as such will return names will all lower case
	f = f.copy()
	f.TableName = TableName(strings.ToUpper(string(f.TableName)))
	f.Last = fieldNameToUpper(f.Last)
	f.Fixed = pure_utils.Map(f.Fixed, fieldNameToUpper)
	f.Flex = set.From(pure_utils.Map(f.Flex.Slice(), fieldNameToUpper))
	f.Others = set.From(pure_utils.Map(f.Others.Slice(), fieldNameToUpper))
	i = ConcreteIndex{
		TableName: TableName(strings.ToUpper(string(i.TableName))),
		Indexed:   pure_utils.Map(i.Indexed, fieldNameToUpper),
		Included:  pure_utils.Map(i.Included, fieldNameToUpper),
	}

	if i.TableName != f.TableName {
		return false
	}

	if len(i.Indexed) < len(f.Fixed) {
		return false
	}

	lastVisited := -1
	// N first items in i.Indexed must be equal to N first elements in f.Fixed (if not empty)
	for n := 0; n < len(f.Fixed) && n < len(i.Indexed); n += 1 {
		if i.Indexed[n] != f.Fixed[n] {
			return false
		}
		lastVisited = n
	}

	// If there are no more elements in f.Fixed but there are some left in f to check, then carry on with the next values in i.Indexed
	lenFlex := f.Flex.Size()
	if lenFlex > 0 {
		start := lastVisited + 1
		if start+lenFlex > len(i.Indexed) {
			return false
		}
		cp := f.Flex.Copy()
		cp.RemoveSlice(i.Indexed[start : start+lenFlex])
		if cp.Size() > 0 {
			return false
		}
	}

	if f.Last != "" {
		if f.size() > len(i.Indexed) || i.Indexed[f.size()-1] != f.Last {
			return false
		}
	}

	if !set.From(i.Included).Subset(f.Others) {
		return false
	}

	return true
}

func fieldNameToUpper(f FieldName) FieldName {
	return FieldName(strings.ToUpper(string(f)))
}

func selectIdxFamiliesToCreate(idxFamilies set.Collection[IndexFamily], existing []ConcreteIndex) set.Collection[IndexFamily] {
	toCreate := set.NewHashSet[IndexFamily](0)

	for _, family := range idxFamilies.Slice() {
		found := false
		for _, concreteIndex := range existing {
			if concreteIndex.covers(family) {
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

func selectConcreteIndexesToCreate(idxFamilies set.Collection[IndexFamily], existing []ConcreteIndex) []ConcreteIndex {
	toCreateFamilies := selectIdxFamiliesToCreate(idxFamilies, existing)

	toCreateIdx := make([]ConcreteIndex, 0, toCreateFamilies.Size())
	toCreateFamilies.ForEach(func(family IndexFamily) bool {
		indexed := family.Fixed
		indexed = append(indexed, family.Flex.Slice()...)
		if family.Last != "" {
			indexed = append(indexed, family.Last)
		}
		toCreateIdx = append(toCreateIdx, ConcreteIndex{
			TableName: family.TableName,
			Indexed:   indexed,
			Included:  family.Others.Slice(),
		})
		return true
	})

	return toCreateIdx
}
