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
