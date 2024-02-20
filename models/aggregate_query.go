package models

import (
	"fmt"
	"slices"

	"github.com/hashicorp/go-set/v2"
)

type AggregateQueryFamily struct {
	TableName               TableName
	EqConditions            *set.Set[FieldName]
	IneqConditions          *set.Set[FieldName]
	SelectOrOtherConditions *set.Set[FieldName]
}

func NewAggregateQueryFamily(tableName string) AggregateQueryFamily {
	return AggregateQueryFamily{
		TableName:               TableName(tableName),
		EqConditions:            set.New[FieldName](0),
		IneqConditions:          set.New[FieldName](0),
		SelectOrOtherConditions: set.New[FieldName](0),
	}
}

func (family AggregateQueryFamily) Equal(other AggregateQueryFamily) bool {
	return family.TableName == other.TableName &&
		family.EqConditions.Equal(other.EqConditions) &&
		family.IneqConditions.Equal(other.IneqConditions) &&
		family.SelectOrOtherConditions.Equal(other.SelectOrOtherConditions)
}

func (family AggregateQueryFamily) Hash() string {
	// Hash function is used for more easily creating a set of unique query families, taking care of deduplication
	var eq, ineq, other string
	if family.EqConditions == nil {
		eq = ""
	} else {
		s := family.EqConditions.Slice()
		slices.Sort(s)
		eq = fmt.Sprintf("%v", s)
	}
	if family.IneqConditions == nil {
		ineq = ""
	} else {
		s := family.IneqConditions.Slice()
		slices.Sort(s)
		ineq = fmt.Sprintf("%v", s)
	}
	if family.SelectOrOtherConditions == nil {
		other = ""
	} else {
		s := family.SelectOrOtherConditions.Slice()
		slices.Sort(s)
		other = fmt.Sprintf("%v", s)
	}
	return fmt.Sprintf("%s - %s - %s - %s", family.TableName, eq, ineq, other)
}

func (qFamily AggregateQueryFamily) ToIndexFamilies() *set.HashSet[IndexFamily, string] {
	// we output a collection of index families, with the different combinations of "inequality filtering"
	//  at the end of the index.
	// E.g. if we have a query with conditions a = 1, b = 2, c > 3, d > 4, e > 5, we output:
	// { Flex: {a,b}, Last: c, Included: {d,e} }  +  { Flex: {a,b}, Last: d, Included: {c,e} }   +  { Flex: {a,b}, Last: e, Included: {c,d} }
	output := set.NewHashSet[IndexFamily, string](0)
	if (qFamily.EqConditions == nil || qFamily.EqConditions.Size() == 0) &&
		(qFamily.IneqConditions == nil || qFamily.IneqConditions.Size() == 0) {
		// if there are no conditions that are indexable, we return an empty family
		return output
	}

	// first iterate on equality conditions and colunms to include anyway
	base := NewIndexFamily()
	base.TableName = qFamily.TableName
	if qFamily.EqConditions != nil {
		qFamily.EqConditions.ForEach(func(f FieldName) bool {
			base.Flex.Insert(f)
			return true
		})
	}
	if qFamily.SelectOrOtherConditions != nil {
		qFamily.SelectOrOtherConditions.ForEach(func(f FieldName) bool {
			base.Included.Insert(f)
			return true
		})
	}
	if qFamily.IneqConditions == nil || qFamily.IneqConditions.Size() == 0 {
		output.Insert(base)
		return output
	}

	// If inequality conditions are involved, we need to create a family for each column involved
	// in the inequality conditions (and complete the "other" columns)
	qFamily.IneqConditions.ForEach(func(f FieldName) bool {
		// we create a copy of the base family
		family := base.Copy()
		// we add the current column as the "last" column
		family.Last = f
		// we add all the other columns as "other" columns
		qFamily.IneqConditions.ForEach(func(o FieldName) bool {
			if o != f {
				family.Included.Insert(o)
			}
			return true
		})
		output.Insert(family)
		return true
	})

	return output
}
