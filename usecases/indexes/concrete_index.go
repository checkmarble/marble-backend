package indexes

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/hashicorp/go-set/v2"
)

func selectConcreteIndexesToCreate(
	idxFamilies set.Collection[models.IndexFamily],
	existing []models.ConcreteIndex,
) []models.ConcreteIndex {
	toCreateFamilies := selectIdxFamiliesToCreate(idxFamilies, existing)

	// this does the projection of index families on real indexes to create
	toCreateIdx := make([]models.ConcreteIndex, 0, toCreateFamilies.Size())
	toCreateFamilies.ForEach(func(family models.IndexFamily) bool {
		indexed := family.Fixed
		// Here is an arbitrary order
		indexed = append(indexed, family.Flex.Slice()...)
		if family.Last != "" {
			indexed = append(indexed, family.Last)
		}
		toCreateIdx = append(toCreateIdx, models.ConcreteIndex{
			TableName: family.TableName,
			Indexed:   indexed,
			Included:  family.Included.Slice(),
			Type:      models.IndexTypeAggregation,
			Status:    models.IndexStatusUnknown,
		})
		return true
	})

	return toCreateIdx
}

// this does the selection part of the job
func selectIdxFamiliesToCreate(
	idxFamilies set.Collection[models.IndexFamily],
	existing []models.ConcreteIndex,
) set.Collection[models.IndexFamily] {
	toCreate := set.NewHashSet[models.IndexFamily](0)

	for _, family := range idxFamilies.Slice() {
		found := false
		for _, concreteIndex := range existing {
			if concreteIndex.Covers(family) {
				found = true
				break
			}
		}
		if !found {
			toCreate.Insert(family.Copy())
		}
	}

	return toCreate
}
