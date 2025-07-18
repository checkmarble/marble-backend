package models

type AiCaseReview struct {
	// True if the output review has been sanity checked
	Ok bool

	// Main body of the review
	Output string

	// Sanity check of the review (if KO)
	SanityCheck string

	// Thought process of the review (if any). Depends on the model used.
	Thought string
}
