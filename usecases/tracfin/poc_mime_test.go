package tracfin

import (
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
)

func TestPOCAttachmentPDFMIME(t *testing.T) {
	c := task5Catalog(t)
	const field = "/ds/pieceJointe/piece/type_mime"
	for _, tc := range []struct {
		mime    string
		allowed bool
	}{
		{"application/pdf", true}, {"image/png", false}, {"application/x-unknown", false}, {"", false},
	} {
		t.Run(tc.mime, func(t *testing.T) {
			piece := task5Block("piece", "/ds/pieceJointe/piece")
			if tc.mime != "" {
				piece.Fields[field] = []models.TracfinValue{task5Text("mime", tc.mime)}
			}
			reports := []models.TracfinValidation{
				ValidateDraft(task5Draft(c, task5Block("pieces", "/ds/pieceJointe", piece)), c),
				ValidateXML(strings.NewReader(`<ds><pieceJointe><piece><type_mime>`+tc.mime+`</type_mime></piece></pieceJointe></ds>`), c),
			}
			for _, report := range reports {
				require.Equal(t, tc.allowed, report.Layers["business"].Status == "passed", "%+v", report.Diagnostics)
				if tc.allowed {
					for _, diag := range report.Diagnostics {
						require.NotEqual(t, "catalog-liste_ref_typemime.csv", diag.RuleID)
					}
				}
			}
		})
	}
	require.Empty(t, ValidateConfiguration(models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "pdf", TargetField: field, Source: models.TracfinMappingSource{Kind: "settings", Field: "mime"}, Codes: []models.TracfinCodeMapping{{Source: "PDF", Target: "application/pdf"}}}}}, c))
	require.Contains(t, c.Codes["liste_ref_typemime.csv"], Code{Value: "application/pdf", Label: "PDF (local POC allowance; official MIME catalogue unavailable)"})
	// The permission is limited to MIME: other unavailable authorities stay unavailable.
	require.Equal(t, "unavailable", c.CodeStatus("missing-catalog.csv", "application/pdf"))
}
