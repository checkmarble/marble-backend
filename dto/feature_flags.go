package dto

type FeatureFlagsDto struct {
	Flags map[string]string `json:"flags" binding:"required"`
}
