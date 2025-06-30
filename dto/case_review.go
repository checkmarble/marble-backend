package dto

type CaseReview struct {
	Ok          bool   `json:"ok"`
	Output      string `json:"output"`
	SanityCheck string `json:"sanity_check,omitempty"`
	Thought     string `json:"thought,omitempty"`
}
