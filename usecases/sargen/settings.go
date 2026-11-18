package sargen

import (
	"context"
	"slices"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats"
)

// settingsSpec returns the spec used to validate the settings of a family: the
// most recent active version of the family.
func (uc *Usecase) settingsSpec(family string) (*core.Spec, error) {
	best, err := settingsFormat(family)
	if err != nil {
		return nil, err
	}
	return best.Spec(), nil
}

// settingsFormat returns the most recent active format of a family.
func settingsFormat(family string) (core.Format, error) {
	var best core.Format
	for _, f := range formats.List() {
		if f.Key().Family != family || f.Status() != core.VersionActive {
			continue
		}
		if best == nil || newerVersion(f.Key().Version, best.Key().Version) {
			best = f
		}
	}
	if best == nil {
		return nil, errors.Wrapf(models.BadParameterError, "famille de format inconnue : %s", family)
	}
	return best, nil
}

// SettingsReadiness is whether the stored organization settings let a
// generation start: Missing are the required paths without a value, Invalid the
// required paths whose value is blocked. Complete is true when both are empty.
// It is the single rule of StartGeneration and of the settings API.
type SettingsReadiness struct {
	Missing  []core.Path
	Invalid  []core.Path
	Complete bool
}

func settingsReadiness(format core.Format, values map[string]string) SettingsReadiness {
	r := SettingsReadiness{Missing: []core.Path{}, Invalid: []core.Path{}}
	spec := format.Spec()
	for _, p := range requiredSettingPaths(format, values) {
		v := values[string(p)]
		if strings.TrimSpace(v) == "" {
			r.Missing = append(r.Missing, p)
			continue
		}
		if issue := core.ValidateValue(spec, p, v); issue != nil && issue.Blocks() {
			r.Invalid = append(r.Invalid, p)
		}
	}
	r.Complete = len(r.Missing) == 0 && len(r.Invalid) == 0
	return r
}

// ReadinessOfSettings evaluates values against the most recent active format of
// the family. StartGeneration evaluates the declaration's own format: both agree
// unless two active versions of the family require different settings.
func ReadinessOfSettings(family string, values map[string]string) (SettingsReadiness, error) {
	format, err := settingsFormat(family)
	if err != nil {
		return SettingsReadiness{}, err
	}
	return settingsReadiness(format, values), nil
}

// SettingsReadiness is ReadinessOfSettings as a usecase method (the API calls it).
func (uc *Usecase) SettingsReadiness(_ context.Context, family string, values map[string]string) (SettingsReadiness, error) {
	return ReadinessOfSettings(family, values)
}

func settingIssues(spec *core.Spec, values map[string]string) []core.Issue {
	paths := make([]string, 0, len(values))
	for p := range values {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	var issues []core.Issue
	for _, p := range paths {
		if issue := core.ValidateValue(spec, core.Path(p), values[p]); issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues
}

// GetOrgSettings returns the organization (from credentials) settings for a
// format family, plus the issues of the stored values.
func (uc *Usecase) GetOrgSettings(ctx context.Context, family string) (models.SarOrgFormatSettings, []core.Issue, error) {
	orgId, err := credsOrg(ctx)
	if err != nil {
		return models.SarOrgFormatSettings{}, nil, err
	}
	spec, err := uc.settingsSpec(family)
	if err != nil {
		return models.SarOrgFormatSettings{}, nil, err
	}
	stored, err := uc.repository.GetSarOrgFormatSettings(ctx, uc.executorFactory.NewExecutor(), orgId, family)
	if err != nil {
		return models.SarOrgFormatSettings{}, nil, err
	}
	if stored == nil {
		return models.SarOrgFormatSettings{OrganizationId: orgId, FormatFamily: family, Values: map[string]string{}}, nil, nil
	}
	return *stored, settingIssues(spec, stored.Values), nil
}

// PutOrgSettings replaces the values. Settings are scoped to the organization of
// the credentials. Any user of the organization may write them (accepted V1
// limit: no finer permission). Allowed paths are fields whose node
// nature is NatureSetting; each value is validated with core.ValidateValue and
// a blocking issue refuses the whole write (BadParameterError).
func (uc *Usecase) PutOrgSettings(ctx context.Context, family string, values map[string]string) (models.SarOrgFormatSettings, []core.Issue, error) {
	orgId, err := credsOrg(ctx)
	if err != nil {
		return models.SarOrgFormatSettings{}, nil, err
	}
	spec, err := uc.settingsSpec(family)
	if err != nil {
		return models.SarOrgFormatSettings{}, nil, err
	}
	for p := range values {
		n := spec.NodeAt(core.Path(p).Template())
		if n == nil || n.Kind != core.NodeField || n.EffectiveNature() != core.NatureSetting {
			return models.SarOrgFormatSettings{}, nil, errors.Wrapf(models.BadParameterError,
				"%s n'est pas un réglage de l'organisation", p)
		}
	}
	issues := settingIssues(spec, values)
	for i := range issues {
		if issues[i].Blocks() {
			return models.SarOrgFormatSettings{}, issues, errors.Wrapf(models.BadParameterError,
				"valeur invalide pour %s : %s", issues[i].Path, issues[i].Message)
		}
	}
	if values == nil {
		values = map[string]string{}
	}
	s := models.SarOrgFormatSettings{OrganizationId: orgId, FormatFamily: family, Values: values}
	if err := uc.repository.UpsertSarOrgFormatSettings(ctx, uc.executorFactory.NewExecutor(), s); err != nil {
		return models.SarOrgFormatSettings{}, nil, err
	}
	return s, issues, nil
}

// newerVersion reports whether version a is strictly newer than b (semantic
// comparison: 1.10.0 is newer than 1.4.0). An unparsable version never wins
// over a parsable one; two unparsable ones are compared as strings.
func newerVersion(a, b string) bool {
	va, errA := semver.NewVersion(a)
	vb, errB := semver.NewVersion(b)
	switch {
	case errA != nil && errB != nil:
		return a > b
	case errA != nil:
		return false
	case errB != nil:
		return true
	}
	return va.GreaterThan(vb)
}
