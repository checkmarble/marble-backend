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

func (f IndexFamily) Copy() IndexFamily {
	return IndexFamily{
		Fixed:  slices.Clone(f.Fixed),
		Flex:   f.Flex.Copy(),
		Last:   f.Last,
		Others: f.Others.Copy(),
	}
}
