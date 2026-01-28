package pure_utils

import "fmt"

// MarbleEntityIdBuilder builds an entity ID in the Marble/OpenSanctions format:
// `marble_<object_type>_<object_id>`.
func MarbleEntityIdBuilder(objectType, objectId string) string {
	return fmt.Sprintf("marble_%s_%s", objectType, objectId)
}
