package models

import (
	"fmt"
	"slices"

	"github.com/hashicorp/go-set/v2"
)

type IndexFamily struct {
	TableName string
	Fixed     []string
	Flex      *set.Set[string]
	Last      string
	Included  *set.Set[string]
}

func NewIndexFamily() IndexFamily {
	return IndexFamily{
		Fixed:    make([]string, 0),
		Flex:     set.New[string](0),
		Last:     "",
		Included: set.New[string](0),
	}
}

func (f IndexFamily) Equal(other IndexFamily) bool {
	return f.TableName == other.TableName &&
		f.Included.Equal(other.Included) &&
		slices.Equal(f.Fixed, other.Fixed) &&
		f.Flex.Equal(other.Flex) &&
		(f.Last == other.Last)
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

func (f IndexFamily) Copy() IndexFamily {
	return IndexFamily{
		TableName: f.TableName,
		Fixed:     slices.Clone(f.Fixed),
		Flex:      f.Flex.Copy(),
		Last:      f.Last,
		Included:  f.Included.Copy(),
	}
}

func (f IndexFamily) AllIndexedValues() set.Collection[string] {
	out := f.Flex.Union(set.From(f.Fixed))
	if f.Last != "" {
		out.Insert(f.Last)
	}
	return out
}

func (f IndexFamily) Size() int {
	s := len(f.Fixed) + f.Flex.Size()
	if f.Last != "" {
		s++
	}
	return s
}

func (f IndexFamily) RemoveFixedPrefix(prefix []string) IndexFamily {
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

func (f IndexFamily) PrependPrefix(prefix []string) IndexFamily {
	return IndexFamily{
		Fixed:    append(prefix, f.Fixed...),
		Flex:     f.Flex.Copy(),
		Last:     f.Last,
		Included: f.Included.Copy(),
	}
}

func (f *IndexFamily) SetLast(last string) {
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

func (f IndexFamily) MergeIncluded(B IndexFamily) IndexFamily {
	out := f.Copy()
	out.Included = out.Included.Union(B.Included).(*set.Set[string])
	out.Included = out.Included.Difference(out.AllIndexedValues()).(*set.Set[string])
	return out
}
