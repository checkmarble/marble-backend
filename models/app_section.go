package models

import "slices"

type AppSection string

const (
	SCENARIOS             AppSection = "scenarios"
	CUSTOMER_SCORING      AppSection = "customer_scoring"
	CONTINUOUS_MONITORING AppSection = "continuous_monitoring"
	CASE_MANAGER          AppSection = "case_manager"
	MANUAL_SCREENING      AppSection = "manual_screening"
	CUSTOMER_HUB          AppSection = "customer_hub"
	DATA_MODEL            AppSection = "data_model"
	SETTINGS              AppSection = "settings"
)

var appSections = []AppSection{
	SCENARIOS, CUSTOMER_SCORING, CONTINUOUS_MONITORING, CASE_MANAGER,
	MANUAL_SCREENING, CUSTOMER_HUB, DATA_MODEL, SETTINGS,
}

var rolesHiddenSections = map[Role][]AppSection{
	ANALYST: {SCENARIOS, CUSTOMER_SCORING, CONTINUOUS_MONITORING, DATA_MODEL, SETTINGS},
}

func AppSections() []AppSection {
	return slices.Clone(appSections)
}

func (s AppSection) IsValid() bool {
	return slices.Contains(appSections, s)
}

func (r Role) HiddenSections() []AppSection {
	return slices.Clone(rolesHiddenSections[r])
}

func SectionsVisibility(hidden []AppSection) map[AppSection]bool {
	sections := make(map[AppSection]bool, len(appSections))

	for _, section := range appSections {
		sections[section] = !slices.Contains(hidden, section)
	}

	return sections
}

func RoleBindingsSections(bindings []RoleBinding, bundle RoleBindingBundle) map[AppSection]bool {
	active := slices.DeleteFunc(slices.Clone(bindings), func(binding RoleBinding) bool {
		return !binding.IsActive(bundle)
	})

	if len(active) == 0 {
		return SectionsVisibility(nil)
	}

	hiddenByAll := slices.Clone(active[0].HiddenSections)

	for _, binding := range active[1:] {
		hiddenByAll = slices.DeleteFunc(hiddenByAll, func(section AppSection) bool {
			return !slices.Contains(binding.HiddenSections, section)
		})
	}

	return SectionsVisibility(hiddenByAll)
}
