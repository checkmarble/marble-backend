package dto

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type Response struct {
	Description string   `json:"description"`
	Content     *Content `json:"content,omitempty"`
}

type Property struct {
	Description *string             `json:"description,omitempty"`
	Type        *string             `json:"type,omitempty"`
	Format      *string             `json:"format,omitempty"`
	Enum        []string            `json:"enum"`
	OneOf       []map[string]string `json:"oneOf,omitempty"`
	Ref         *string             `json:"$ref,omitempty"`
	Items       *Schema             `json:"items,omitempty"`
}

type Schema struct {
	Ref string `json:"$ref"`
}

type ApplicationJSON struct {
	Schema Schema `json:"schema"`
}

type Content struct {
	ApplicationJSON ApplicationJSON `json:"application/json"`
}

type RequestBody struct {
	Content  Content `json:"content"`
	Required bool    `json:"required"`
}

type MethodObject struct {
	Security    []map[string][]string `json:"security"`
	Tags        []string              `json:"tags"`
	Description string                `json:"description"`
	RequestBody RequestBody           `json:"requestBody"`
	Responses   map[string]Response   `json:"responses"`
}

type PathObject struct {
	Post MethodObject `json:"post"`
}

type ComponentsSchema struct {
	Required   []string            `json:"required,omitempty"`
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
}

type APIKey struct {
	Type string `json:"type"`
	Name string `json:"name"`
	In   string `json:"in"`
}

type SecuritySchemes struct {
	APIKey APIKey `json:"api_key"`
}

type Components struct {
	Schemas         map[string]ComponentsSchema `json:"schemas"`
	SecuritySchemes SecuritySchemes             `json:"securitySchemes"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Security struct {
	ApiKey []string `json:"api_key"`
}

type Reference struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Tags       []Tag                 `json:"tags"`
	Paths      map[string]PathObject `json:"paths"`
	Components Components            `json:"components"`
}

func toSwaggerType(t models.DataType) *string {
	switch t {
	case models.Int:
		return utils.Ptr("integer")
	case models.Float:
		return utils.Ptr("number")
	case models.String, models.Timestamp:
		return utils.Ptr("string")
	case models.Bool:
		return utils.Ptr("boolean")
	}
	return utils.Ptr("object")
}

func decisionInputSchema(triggerObjects []map[string]string) ComponentsSchema {
	return ComponentsSchema{
		Required: []string{
			"scenario_id",
			"trigger_object",
			"object_type",
		},
		Type: "object",
		Properties: map[string]Property{
			"scenario_id": {
				Description: utils.Ptr("ID of the scenario used to take a decision."),
				Type:        utils.Ptr("string"),
			},
			"trigger_object": {
				Description: utils.Ptr("The object used to take a decision."),
				Type:        utils.Ptr("object"),
				OneOf:       triggerObjects,
			},
			"object_type": {
				Description: utils.Ptr("The object type used to take a decision."),
				Type:        utils.Ptr("string"),
			},
		},
	}
}

func decisionSchema(triggerObjects []map[string]string) ComponentsSchema {
	return ComponentsSchema{
		Type: "object",
		Properties: map[string]Property{
			"id": {
				Description: utils.Ptr("ID of the returned decision."),
				Type:        utils.Ptr("string"),
			},
			"app_link": {
				Description: utils.Ptr("Link to the decision in the app"),
				Type:        utils.Ptr("string"),
			},
			"created_at": {
				Description: utils.Ptr("Date the decision was taken."),
				Type:        utils.Ptr("string"),
				Format:      utils.Ptr("date-time"),
			},
			"trigger_object": {
				Description: utils.Ptr("Object used to take a decision."),
				Type:        utils.Ptr("object"),
				OneOf:       triggerObjects,
			},
			"trigger_object_type": {
				Description: utils.Ptr("Object type used to take a decision."),
				Type:        utils.Ptr("string"),
			},
			"outcome": {
				Description: utils.Ptr("Object type used to take a decision."),
				Type:        utils.Ptr("string"),
				Enum:        []string{"approve", "review", "decline"},
			},
			"scenario": {
				Ref: utils.Ptr("#/components/schemas/Scenario"),
			},
			"rules": {
				Description: utils.Ptr(""),
				Type:        utils.Ptr("array"),
				Items: &Schema{
					Ref: "#/components/schemas/DecisionRule",
				},
			},
			"score": {
				Description: utils.Ptr("Score of the decision."),
				Type:        utils.Ptr("integer"),
			},
			"error": {
				Ref: utils.Ptr("#/components/schemas/Error"),
			},
			"scheduled_execution_id": {
				Description: utils.Ptr("Id of the scheduled execution, if the decision was created during a batch execution."),
				Type:        utils.Ptr("string"),
			},
			"pivot_values": {
				Description: utils.Ptr("Array (0 or 1 elements) of the pivot values attached to the decision."),
				Type:        utils.Ptr("array"),
				Items:       &Schema{Ref: "#/components/schemas/PivotValues"},
			},
		},
	}
}

func decisionRuleSchema() ComponentsSchema {
	return ComponentsSchema{
		Type: "object",
		Properties: map[string]Property{
			"name": {
				Type: utils.Ptr("string"),
			},
			"description": {
				Type: utils.Ptr("string"),
			},
			"score_modifier": {
				Type: utils.Ptr("integer"),
			},
			"result": {
				Type: utils.Ptr("boolean"),
			},
			"error": {
				Ref: utils.Ptr("#/components/schemas/Error"),
			},
			"rule_id": {
				Type:        utils.Ptr("string"),
				Description: utils.Ptr("Id of the rule used for this rule execution."),
			},
			"outcome": {
				Type: utils.Ptr("string"),
				Enum: []string{"hit", "no_hit", "error", "snoozed"},
			},
		},
	}
}

func scenarioSchema() ComponentsSchema {
	return ComponentsSchema{
		Type: "object",
		Properties: map[string]Property{
			"id": {
				Type: utils.Ptr("string"),
			},
			"name": {
				Type: utils.Ptr("string"),
			},
			"description": {
				Type: utils.Ptr("string"),
			},
			"version": {
				Type: utils.Ptr("integer"),
			},
		},
	}
}

func pivotValuesSchema() ComponentsSchema {
	return ComponentsSchema{
		Type: "object",
		Properties: map[string]Property{
			"pivot_id": {
				Type: utils.Ptr("string"),
			},
			"pivot_value": {
				Type: utils.Ptr("string"),
			},
		},
	}
}

func errorSchema() ComponentsSchema {
	return ComponentsSchema{
		Type: "object",
		Properties: map[string]Property{
			"code": {
				Type: utils.Ptr("integer"),
			},
			"message": {
				Type: utils.Ptr("string"),
			},
		},
	}
}

func OpenAPIFromDataModel(dataModel models.DataModel) Reference {
	ref := Reference{
		OpenAPI: "3.0.3",
		Info:    Info{},
		Tags: []Tag{
			{
				Name:        "Ingestion",
				Description: "Ingestion routes for your data model",
			},
			{
				Name:        "Decisions",
				Description: "Decisions route for your data model",
			},
		},
	}

	ref.Paths = make(map[string]PathObject)
	ref.Components.Schemas = make(map[string]ComponentsSchema)

	for _, table := range dataModel.Tables {
		var required []string
		properties := make(map[string]Property)
		for name, field := range table.Fields {
			description := field.Description
			properties[name] = Property{
				Description: &description,
				Type:        toSwaggerType(field.DataType),
			}
			if !field.Nullable {
				required = append(required, name)
			}
		}

		ref.Components.Schemas[table.Name] = ComponentsSchema{
			Required:   required,
			Type:       "object",
			Properties: properties,
		}

		object := PathObject{
			Post: MethodObject{
				Security: []map[string][]string{
					{
						"api_key": []string{},
					},
				},
				Tags:        []string{"Ingestion"},
				Description: table.Description,
				RequestBody: RequestBody{
					Content: Content{
						ApplicationJSON: ApplicationJSON{
							Schema: Schema{
								Ref: fmt.Sprintf("#/components/schemas/%s", table.Name),
							},
						},
					},
					Required: true,
				},
				Responses: map[string]Response{
					"200": {
						Description: "data was successfully ingested",
					},
					"500": {
						Description: "an error happened while ingesting data",
					},
				},
			},
		}
		ref.Paths[fmt.Sprintf("/ingestion/%s", table.Name)] = object
	}

	var triggerObjects []map[string]string
	for _, table := range dataModel.Tables {
		ref := map[string]string{
			"$ref": fmt.Sprintf("#/components/schemas/%s", table.Name),
		}
		triggerObjects = append(triggerObjects, ref)
	}

	ref.Components.Schemas["Error"] = errorSchema()
	ref.Components.Schemas["Scenario"] = scenarioSchema()
	ref.Components.Schemas["PivotValues"] = pivotValuesSchema()
	ref.Components.Schemas["DecisionInput"] = decisionInputSchema(triggerObjects)
	ref.Components.Schemas["Decision"] = decisionSchema(triggerObjects)
	ref.Components.Schemas["DecisionRule"] = decisionRuleSchema()
	ref.Components.SecuritySchemes = SecuritySchemes{
		APIKey: APIKey{
			Type: "apiKey",
			Name: "X-API-Key",
			In:   "header",
		},
	}

	ref.Paths["/decisions"] = PathObject{
		Post: MethodObject{
			Security: []map[string][]string{
				{
					"api_key": []string{},
				},
			},
			Tags:        []string{"Decisions"},
			Description: "Decisions API",
			RequestBody: RequestBody{
				Content: Content{
					ApplicationJSON: ApplicationJSON{
						Schema: Schema{
							Ref: "#/components/schemas/DecisionInput",
						},
					},
				},
				Required: true,
			},
			Responses: map[string]Response{
				"200": {
					Description: "a decision was successfully returned",
					Content: &Content{
						ApplicationJSON: ApplicationJSON{
							Schema: Schema{
								Ref: "#/components/schemas/Decision",
							},
						},
					},
				},
				"500": {
					Description: "an error happened while taking a decision",
				},
			},
		},
	}
	return ref
}
