package tracfin

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func conditionalBlock(path string, fields map[string]string, children ...models.TracfinBlock) models.TracfinBlock {
	b := models.TracfinBlock{ID: path, CatalogID: path, Fields: map[string][]models.TracfinValue{}, Children: children}
	for name, text := range fields {
		s := text
		b.Fields[path+"/"+name] = []models.TracfinValue{{ID: name, Kind: "text", Text: &s}}
	}
	return b
}
func conditionalDiagnostics(t *testing.T, b models.TracfinBlock) []models.TracfinDiagnostic {
	t.Helper()
	c := task5Catalog(t)
	v := newValidation(c)
	validateConditionalRules(&v, b, c)
	return v.Diagnostics
}
func hasConditionalField(ds []models.TracfinDiagnostic, field string) bool {
	for _, d := range ds {
		if d.Field != nil && strings.HasSuffix(d.Field.FieldID, "/"+field) {
			return true
		}
	}
	return false
}
func TestConditionalIdentityAndDateAlternatives(t *testing.T) {
	for _, path := range []string{"/ds/personnes/pp/etatCivil", "/ds/personnes/pp/etatCivil/autreIdentite"} {
		ds := conditionalDiagnostics(t, conditionalBlock(path, nil))
		require.True(t, hasConditionalField(ds, "nomDusage"))
		require.True(t, hasConditionalField(ds, "alias"))
		require.Empty(t, conditionalDiagnostics(t, conditionalBlock(path, map[string]string{"nomDusage": "Dupont", "alias": "Jean"})))
		ds = conditionalDiagnostics(t, conditionalBlock(path, map[string]string{"nomNaissance": "Dupont", "prenom": "Jean", "dateNaissance": "01/01/1980", "dateNaissanceEstimation": "1980"}))
		require.True(t, hasConditionalField(ds, "dateNaissanceEstimation"))
	}
}
func TestConditionalRegisterRules(t *testing.T) {
	for _, path := range []string{"/ds/personnes/pp/activite", "/ds/personnes/pm/activiteCoordonnee", "/ds/personnes/pm/dirigeant/autreDirigeantM"} {
		b := conditionalBlock(path, map[string]string{"numeroIdentification": "123", "numeroIdentification/@type": "SIREN"})
		ds := conditionalDiagnostics(t, b)
		require.True(t, hasConditionalField(ds, "denominationSociale"))
		require.True(t, hasConditionalField(ds, "secteurActivite"))
		b = conditionalBlock(path, map[string]string{"numeroIdentification": "123", "numeroIdentification/@type": "RN6", "denominationSociale": "Foreign company"})
		ds = conditionalDiagnostics(t, b)
		require.True(t, hasConditionalField(ds, "nomAutreTypeRegistre"))
		require.True(t, hasConditionalField(ds, "paysAutreTypeRegistre"))
		b = conditionalBlock(path, map[string]string{"numeroIdentification": "", "numeroIdentification/@type": "Je ne sais pas"})
		require.Empty(t, conditionalDiagnostics(t, b))
	}
}
func TestConditionalOperationParties(t *testing.T) {
	path := "/ds/operations/operation"
	fields := map[string]string{"typeOperation": "OT03"}
	ds := conditionalDiagnostics(t, conditionalBlock(path, fields))
	for _, name := range []string{"emetteurTypeEmetteur", "paysOrigineFonds", "refFinanciereEmission", "beneficiaireTypeBeneficiaire", "paysArriveeFonds", "refFinanciereReception"} {
		require.True(t, hasConditionalField(ds, name), name)
	}
	fields["typeOperation"] = "OT07"
	ds = conditionalDiagnostics(t, conditionalBlock(path, fields))
	require.False(t, hasConditionalField(ds, "emetteurTypeEmetteur"))
	require.True(t, hasConditionalField(ds, "beneficiaireTypeBeneficiaire"))
	fields["typeOperation"] = "OT06"
	ds = conditionalDiagnostics(t, conditionalBlock(path, fields))
	require.True(t, hasConditionalField(ds, "emetteurTypeEmetteur"))
	require.False(t, hasConditionalField(ds, "beneficiaireTypeBeneficiaire"))
}
func TestConditionalLegalGrounds(t *testing.T) {
	path := "/ds/context/natureSoupcon"
	ds := conditionalDiagnostics(t, conditionalBlock(path, map[string]string{"cadreLegal": "CL02"}))
	require.True(t, hasConditionalField(ds, "natureSoupconPenale"))
	ds = conditionalDiagnostics(t, conditionalBlock(path, map[string]string{"cadreLegal": "CL03", "natureFraudeFiscale": "NFF10", "natureSoupconPenale": "NSP15"}))
	require.True(t, hasConditionalField(ds, "autreNatureFraudeFiscale"))
	require.True(t, hasConditionalField(ds, "natureSoupconPenale"))
	require.Empty(t, conditionalDiagnostics(t, conditionalBlock(path, map[string]string{"cadreLegal": "CL01"})))
	ds = conditionalDiagnostics(t, conditionalBlock(path, nil))
	require.True(t, hasConditionalField(ds, "cadreLegal"))
}
func TestConditionalDirectorSanctionsAndPeople(t *testing.T) {
	d := "/ds/personnes/pm/dirigeant"
	ds := conditionalDiagnostics(t, conditionalBlock(d, map[string]string{"fonctionDirigeant": "MF14", "beneficiaireEffectif": "Non"}))
	require.True(t, hasConditionalField(ds, "autreFonctionDirigeant"))
	require.True(t, hasConditionalField(ds, "idBeneficiaireEffectif"))
	b := conditionalBlock(d, map[string]string{"beneficiaireEffectif": "Non", "idBeneficiaireEffectif": "1"}, conditionalBlock(d+"/autreBeneficiaireEffectif", nil))
	require.True(t, hasConditionalField(conditionalDiagnostics(t, b), "idBeneficiaireEffectif"))
	n := "/ds/personnes/pp/notorieteDefavorable"
	require.True(t, hasConditionalField(conditionalDiagnostics(t, conditionalBlock(n, map[string]string{"presenceSanction": "false", "listesSanctions": "list"})), "listesSanctions"))
	require.Empty(t, conditionalDiagnostics(t, conditionalBlock(n, map[string]string{"presenceSanction": "true", "listesSanctions": "list"})))
	require.True(t, hasConditionalField(conditionalDiagnostics(t, conditionalBlock("/ds/personnes", nil)), "personnes"))
}
func TestConditionalBankFieldsRespectFalseAndProfession(t *testing.T) {
	makeRoot := func(profession string, fields map[string]string) models.TracfinBlock {
		return conditionalBlock("/ds", nil, conditionalBlock("/ds/organisme", map[string]string{"profession": profession}), conditionalBlock("/ds/context", nil, conditionalBlock("/ds/context/information", fields)))
	}
	ds := conditionalDiagnostics(t, makeRoot("TPI1", nil))
	require.True(t, hasConditionalField(ds, "gestionFortuneBanquePrivee"))
	require.True(t, hasConditionalField(ds, "correspondanceBancaire"))
	require.Empty(t, conditionalDiagnostics(t, makeRoot("TPI1", map[string]string{"gestionFortuneBanquePrivee": "false", "correspondanceBancaire": "false"})))
	require.Empty(t, conditionalDiagnostics(t, makeRoot("TPI10", nil)))
}

func TestConditionalOperationMissingTypeDoesNotGrantCashExceptions(t *testing.T) {
	ds := conditionalDiagnostics(t, conditionalBlock("/ds/operations/operation", map[string]string{"montantTotal": "100"}))
	for _, field := range []string{"emetteurTypeEmetteur", "paysOrigineFonds", "refFinanciereEmission", "beneficiaireTypeBeneficiaire", "paysArriveeFonds", "refFinanciereReception"} {
		require.True(t, hasConditionalField(ds, field), field)
	}
	c := task5Catalog(t)
	xml := `<ds><operations><montantTotal>100</montantTotal><operation><montantTotal>100</montantTotal></operation></operations></ds>`
	report := ValidateXML(strings.NewReader(xml), c)
	require.True(t, hasConditionalField(report.Diagnostics, "emetteurTypeEmetteur"))
	require.True(t, hasConditionalField(report.Diagnostics, "beneficiaireTypeBeneficiaire"))
}
func TestConditionalUnknownRegistryAllowsOptionalBusinessDetails(t *testing.T) {
	for _, path := range []string{"/ds/personnes/pp/activite", "/ds/personnes/pm/activiteCoordonnee"} {
		fields := map[string]string{"numeroIdentification": "", "numeroIdentification/@type": "Je ne sais pas", "formeJuridique": "known form", "activitePrincipale": "known activity"}
		if strings.HasSuffix(path, "activiteCoordonnee") {
			fields["enseigneSigle"] = "known brand"
		}
		require.Empty(t, conditionalDiagnostics(t, conditionalBlock(path, fields)))
	}
}

func TestConditionalOptionalFieldsAndOtherValues(t *testing.T) {
	for _, path := range []string{"/ds/personnes/pp/activite", "/ds/personnes/pm/activiteCoordonnee"} {
		bad := conditionalBlock(path, map[string]string{"numeroIdentification": "123", "numeroIdentification/@type": "SIREN", "denominationSociale": "Company", "secteurActivite": "Sector", "formeJuridique": "Form", "activitePrincipale": "Activity"})
		ds := conditionalDiagnostics(t, bad)
		require.True(t, hasConditionalField(ds, "formeJuridique"))
		require.True(t, hasConditionalField(ds, "activitePrincipale"))
		bad = conditionalBlock(path, map[string]string{"numeroIdentification": "123", "numeroIdentification/@type": "RN1", "denominationSociale": "Company"}, conditionalBlock(path+"/autreAdresse", nil))
		require.True(t, hasConditionalField(conditionalDiagnostics(t, bad), "autreAdresse"))
	}
	rel := "/ds/personnes/pm/lienAutrePp/typeRelation"
	require.Empty(t, conditionalDiagnostics(t, conditionalBlock(rel, map[string]string{"sousCategorieRelation": "TLP11", "sousCategorieRelationAutre": "Other family relation"})))
	require.True(t, hasConditionalField(conditionalDiagnostics(t, conditionalBlock(rel, map[string]string{"sousCategorieRelation": "TLP1", "sousCategorieRelationAutre": "Unexpected detail"})), "sousCategorieRelationAutre"))
	p := "/ds/context/natureSoupcon"
	require.Empty(t, conditionalDiagnostics(t, conditionalBlock(p, map[string]string{"cadreLegal": "CL02", "natureSoupconPenale": "NSP15", "autreNatureSoupconPenale": "Detailed suspicion"})))
	require.True(t, hasConditionalField(conditionalDiagnostics(t, conditionalBlock(p, map[string]string{"cadreLegal": "CL02", "natureSoupconPenale": "NSP15"})), "autreNatureSoupconPenale"))
}
func TestConditionalRulesHaveXMLParity(t *testing.T) {
	c := task5Catalog(t)
	report := ValidateXML(strings.NewReader(`<ds><organisme><profession>TPI1</profession></organisme><context><natureSoupcon><cadreLegal>CL02</cadreLegal></natureSoupcon><information/></context><personnes><pp><etatCivil/></pp></personnes></ds>`), c)
	for _, field := range []string{"nomDusage", "alias", "natureSoupconPenale", "gestionFortuneBanquePrivee", "correspondanceBancaire"} {
		require.True(t, hasConditionalField(report.Diagnostics, field), field)
	}
	// A newly declared beneficiary satisfies the alternative without an existing-person reference.
	b := conditionalBlock("/ds/personnes/pm/dirigeant", map[string]string{"beneficiaireEffectif": "Non"}, conditionalBlock("/ds/personnes/pm/dirigeant/autreBeneficiaireEffectif", nil))
	require.False(t, hasConditionalField(conditionalDiagnostics(t, b), "idBeneficiaireEffectif"))
}
