package models

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set/v2"
)

const (
	MAX_POSTGRES_INDEX_NAME_LENGTH      = 63
	MAX_INDEX_NAME_LENGTH_BEFORE_SUFFIX = 53
)

type UnicityIndex struct {
	TableName         string
	Fields            []string
	Included          []string
	CreationInProcess bool
}

type ConcreteIndex struct {
	TableName string
	name      string
	Indexed   []string
	Included  []string
	Type      IndexType
	Status    IndexStatus
}

// Custom marshaller to ensure the name is initialized before marshaling
func (i ConcreteIndex) MarshalJSON() ([]byte, error) {
	// Ensure the name is initialized before marshaling
	i.setName()
	type concreteIndexJSON struct {
		TableName string   `json:"TableName"` //nolint:tagliatelle
		IndexName string   `json:"IndexName"` //nolint:tagliatelle
		Indexed   []string `json:"Indexed"`   //nolint:tagliatelle
		Included  []string `json:"Included"`  //nolint:tagliatelle
	}
	return json.Marshal(concreteIndexJSON{
		TableName: i.TableName,
		IndexName: i.name,
		Indexed:   i.Indexed,
		Included:  i.Included,
	})
}

// custom unmarshaller to avoid name mismatches
func (i *ConcreteIndex) UnmarshalJSON(data []byte) error {
	var rawData struct {
		TableName string   `json:"TableName"` //nolint:tagliatelle
		IndexName string   `json:"IndexName"` //nolint:tagliatelle
		Indexed   []string `json:"Indexed"`   //nolint:tagliatelle
		Included  []string `json:"Included"`  //nolint:tagliatelle
	}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return err
	}
	i.TableName = rawData.TableName
	i.name = rawData.IndexName
	i.Indexed = rawData.Indexed
	i.Included = rawData.Included

	return nil
}

func (i *ConcreteIndex) setName() {
	if i.name != "" {
		return
	}
	// postgresql enforces a 63 character length limit on all identifiers
	prefix := ""
	switch i.Type {
	case IndexTypeNavigation:
		prefix = "nav"
	case IndexTypeAggregation:
		prefix = "idx"
	default:
		prefix = "idx"
	}

	indexedNames := strings.Join(i.Indexed, "-")
	out := fmt.Sprintf("%s_%s_%s", prefix, i.TableName, indexedNames)
	randomId := uuid.NewString()
	length := min(len(out), MAX_INDEX_NAME_LENGTH_BEFORE_SUFFIX)

	withRandomSuffix := out[:length] + "_" + randomId
	i.name = withRandomSuffix[:min(len(withRandomSuffix), MAX_POSTGRES_INDEX_NAME_LENGTH)]
}

// Returns the name of the index. Is generated based on the indexed table name and the indexed columns.
// Is suffixed with a random id to avoid collisions when creating indexes on postgres (e.g. if a previous version
// of the index exists but is invalid).
// Includes logic to truncate the index name length to 63 chars max, so that name truncation by Postgres does not
// interfere with the index name comparison logic.
func (i *ConcreteIndex) Name() string {
	if i.name == "" {
		i.setName()
	}
	return i.name
}

func (i ConcreteIndex) WithName(name string) ConcreteIndex {
	i.name = name
	return i
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
	f.TableName = strings.ToUpper(f.TableName)
	f.Last = strings.ToUpper(f.Last)
	f.Fixed = pure_utils.Map(f.Fixed, strings.ToUpper)
	f.Flex = set.From(pure_utils.Map(f.Flex.Slice(), strings.ToUpper))
	f.Included = set.From(pure_utils.Map(f.Included.Slice(), strings.ToUpper))
	i = ConcreteIndex{
		TableName: strings.ToUpper(i.TableName),
		Indexed:   pure_utils.Map(i.Indexed, strings.ToUpper),
		Included:  pure_utils.Map(i.Included, strings.ToUpper),
		// variable is local to this function, no need to set the other fields
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

type IndexStatus int

const (
	IndexStatusUnknown IndexStatus = iota
	IndexStatusPending
	IndexStatusValid
	IndexStatusInvalid
)

func (s IndexStatus) String() string {
	switch s {
	case IndexStatusPending:
		return "pending"
	case IndexStatusValid:
		return "valid"
	case IndexStatusInvalid:
		return "invalid"
	default:
		return "unknown"
	}
}

func IndexStatusFromString(s string) IndexStatus {
	switch s {
	case "pending":
		return IndexStatusPending
	case "valid":
		return IndexStatusValid
	case "invalid":
		return IndexStatusInvalid
	default:
		return IndexStatusUnknown
	}
}

type IndexType int

const (
	IndexTypeUnknown IndexType = iota
	IndexTypeNavigation
	IndexTypeAggregation
)

// TODO: remove it not used at the end of the PR
// func (t IndexType) String() string {
// 	switch t {
// 	case IndexTypeNavigation:
// 		return "navigation"
// 	case IndexTypeAggregation:
// 		return "aggregation"
// 	default:
// 		return "unknown"
// 	}
// }

// func IndexTypeFromString(s string) IndexType {
// 	switch s {
// 	case "navigation":
// 		return IndexTypeNavigation
// 	case "aggregation":
// 		return IndexTypeAggregation
// 	default:
// 		return IndexTypeUnknown
// 	}
// }
