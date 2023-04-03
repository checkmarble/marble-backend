package api

type APIDecision struct {
	ID             string              `json:"id"`
	Created_at     int64               `json:"created_at"`
	Trigger_object map[string]any      `json:"trigger_object"`
	Outcome        string              `json:"outcome"`
	Scenario       APIDecisionScenario `json:"scenario"`
	Rules          []APIDecisionRule   `json:"rules"`
	Score          int                 `json:"score"`
	Error          *APIError           `json:"error"`
}

type APIDecisionScenario struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     int    `json:"version"`
}

type APIDecisionRule struct {
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	ScoreModifier int       `json:"score_modifier"`
	Result        bool      `json:"result"`
	Error         *APIError `json:"error"`
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
