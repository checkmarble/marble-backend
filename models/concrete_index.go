package models

import (
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/hashicorp/go-set/v2"
)

type UnicityIndex struct {
	TableName         TableName
	Fields            []FieldName
	CreationInProcess bool
}

type ConcreteIndex struct {
	TableName TableName
	Indexed   []FieldName
	Included  []FieldName
}

func (i ConcreteIndex) Equal(other ConcreteIndex) bool {
	return i.TableName == other.TableName &&
		slices.Equal(i.Indexed, other.Indexed) &&
		set.From(i.Included).Equal(set.From(other.Included))
}

func (i ConcreteIndex) Covers(f IndexFamily) bool {
	// We need to make a copy of f because we are going to modify it
	// put all the identifiers to upper case, because postgres is not case sensitive as
	// far as identifiers are concerned (unless quoted)
	// and as such will return names will all lower case
	f = f.Copy()
	f.TableName = TableName(strings.ToUpper(string(f.TableName)))
	f.Last = fieldNameToUpper(f.Last)
	f.Fixed = pure_utils.Map(f.Fixed, fieldNameToUpper)
	f.Flex = set.From(pure_utils.Map(f.Flex.Slice(), fieldNameToUpper))
	f.Included = set.From(pure_utils.Map(f.Included.Slice(), fieldNameToUpper))
	i = ConcreteIndex{
		TableName: TableName(strings.ToUpper(string(i.TableName))),
		Indexed:   pure_utils.Map(i.Indexed, fieldNameToUpper),
		Included:  pure_utils.Map(i.Included, fieldNameToUpper),
	}

	if i.TableName != f.TableName {
		return false
	}

	// index:        [x1, x2, x3, x4]
	// index family: [x1, x2, x3, x4, x5, x6, x7, x8, x9, x10] {...}
	// does not cover: some fields must be indexed in the index family but are not in the concrete index
	if len(i.Indexed) < len(f.Fixed) {
		return false
	}

	lastVisited := -1
	// N first items in i.Indexed must be equal to N first elements in f.Fixed (if not empty)
	// index:        [x1, x2, X3, x4, x5, x6, x7, x8, x9, x10]
	// index family: [x1, x2, Y3, x4] {...}
	//                        ^^
	// The 3rd element in the index family is not equal to the 3rd element in the concrete index
	for n := 0; n < len(f.Fixed) && n < len(i.Indexed); n += 1 {
		if i.Indexed[n] != f.Fixed[n] {
			return false
		}
		lastVisited = n
	}

	// If there are no more elements in f.Fixed but there are some left in f to check, then carry on with the next values in i.Indexed
	// index:        [x1, x2,  x3, x4, x5, x7, x8, x9] => no x6 between indexes 2 and 6
	// index family: [x1, x2] {x3, x6, x4, x5, x7}     "" {}
	//                             ^^
	// does not cover: no x6 between indexes 2 and 6
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

	// index:         [x1, x2,  x3, x4, x5, x6, x7, x8, x9]
	// index family:  [x1, x2] {x3, x4, x5, x6, x7} x9
	//                                              ^^
	// does not cover: x8 is not in the index family but comes before x9 in the concrete index
	if f.Last != "" {
		if f.Size() > len(i.Indexed) || i.Indexed[f.Size()-1] != f.Last {
			return false
		}
	}

	// included columns in the concrete index are missing some required included columns in the index family
	if !set.From(i.Included).Subset(f.Included) {
		return false
	}

	return true
}

func fieldNameToUpper(f FieldName) FieldName {
	return FieldName(strings.ToUpper(string(f)))
}
