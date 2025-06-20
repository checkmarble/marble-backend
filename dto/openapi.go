package dto

import (
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/getkin/kin-openapi/openapi3"
)

func OpenAPIFromDataModel(dataModel models.DataModel, spec *openapi3.T) (*openapi3.T, error) {
	for _, table := range dataModel.Tables {
		fields, required := objectFields(table)

		spec.Paths.Set(fmt.Sprintf("/ingest/%s", table.Name), &openapi3.PathItem{
			Post:  ingestOp(spec, false, false, table, fields, required),
			Patch: ingestOp(spec, false, true, table, fields, required),
		})

		spec.Paths.Set(fmt.Sprintf("/ingest/%s/batch", table.Name), &openapi3.PathItem{
			Post:  ingestOp(spec, true, false, table, fields, required),
			Patch: ingestOp(spec, true, true, table, fields, required),
		})
	}

	return spec, nil
}

func ingestOp(spec *openapi3.T, isBatch, isPatch bool, table models.Table, fields openapi3.Schemas, required []string) *openapi3.Operation {
	summary := fmt.Sprintf("Ingest an object into '%s'", table.Name)
	if isBatch {
		summary = fmt.Sprintf("Ingest multiple objects into '%s'", table.Name)
	}

	security := openapi3.NewSecurityRequirement()
	for securityScheme := range spec.Components.SecuritySchemes {
		security = security.Authenticate(securityScheme)
	}

	return &openapi3.Operation{
		Summary:  summary,
		Tags:     []string{fmt.Sprintf("Ingest %s", table.Name)},
		Security: openapi3.NewSecurityRequirements().With(security),
		RequestBody: &openapi3.RequestBodyRef{
			Value: openapi3.NewRequestBody().
				WithRequired(true).
				WithContent(openapi3.NewContentWithJSONSchema(requestBody(isBatch, isPatch, fields, required)))},
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse().WithDescription("Data was processed but no new object was ingested")),
			openapi3.WithName("201", openapi3.NewResponse().WithDescription("Data was successfully ingested")),
			openapi3.WithName("400",
				openapi3.NewResponse().
					WithDescription("The array of objects is too long, or one object did not match the data model").
					WithContent(openapi3.NewContentWithJSONSchemaRef(openapi3.NewSchemaRef("#/components/schemas/ErrorDto", nil)))),
			openapi3.WithName("500", openapi3.NewResponse().WithDescription("An error happened while ingesting data")),
		),
	}
}

func objectFields(table models.Table) (openapi3.Schemas, []string) {
	fields := make(openapi3.Schemas, len(table.Fields))
	required := make([]string, 0, len(table.Fields))

	for _, field := range table.Fields {
		types := openapi3.Types{}
		format := ""

		if !field.Nullable {
			required = append(required, field.Name)
		}

		switch field.DataType {
		case models.String:
			types = openapi3.Types{openapi3.TypeString}
		case models.Int:
			types = openapi3.Types{openapi3.TypeInteger}
		case models.Float:
			types = openapi3.Types{openapi3.TypeNumber}
		case models.Timestamp:
			types = openapi3.Types{openapi3.TypeString}
			format = "date-time"
		}

		fields[field.Name] = &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Description: field.Description,
				Type:        &types,
				Format:      format,
			},
		}
	}

	return fields, required
}

func requestBody(isBatch, isPatch bool, fields openapi3.Schemas, required []string) *openapi3.Schema {
	object := objectSchema(isPatch, fields, required)

	if !isBatch {
		return &object
	}

	return &openapi3.Schema{
		Type:     &openapi3.Types{openapi3.TypeArray},
		MaxItems: utils.Ptr(uint64(100)),
		Items:    &openapi3.SchemaRef{Value: &object},
	}
}

func objectSchema(isPatch bool, fields openapi3.Schemas, required []string) openapi3.Schema {
	if isPatch {
		required = []string{"object_id", "updated_at"}
	}

	slices.Sort(required)

	return openapi3.Schema{
		Type:       &openapi3.Types{openapi3.TypeObject},
		Required:   required,
		Properties: fields,
	}
}
