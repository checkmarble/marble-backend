package evaluate

import "marble/marble-backend/models"

const TestListId string = "1"
const TestListOrgId string = "2"

var TestList models.CustomList = models.CustomList{
	Id:    TestListId,
	OrgId: TestListOrgId,
}

var TestNamedArgs = map[string]any{
	"customListId": TestListId,
}
