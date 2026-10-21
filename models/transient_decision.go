package models

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/klauspost/compress/zstd"
)

var (
	transientDecisionEncoder, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
	transientDecisionDecoder, _ = zstd.NewReader(nil)
)

// TransientDecision is the minimal representation of a decision that was evaluated but not yet
// persisted, used for asynchronous storage. It only contains what is required to store the
// decision and to present it, rules being stripped of their formula.
type TransientDecision struct {
	Decision          DecisionWithRuleExecutions
	AnalyticsFields   map[string]any
	ObjectId          string
	TriggerObjectType string
}

func NewTransientDecision(bundle DecisionBundle) TransientDecision {
	decision := bundle.Decision
	decision.RuleExecutions = make([]RuleExecution, len(bundle.Decision.RuleExecutions))

	for i, re := range bundle.Decision.RuleExecutions {
		re.Rule = Rule{
			Id:            re.Rule.Id,
			StableRuleId:  re.Rule.StableRuleId,
			Name:          re.Rule.Name,
			Description:   re.Rule.Description,
			ScoreModifier: re.Rule.ScoreModifier,
			RuleGroup:     re.Rule.RuleGroup,
		}

		decision.RuleExecutions[i] = re
	}

	return TransientDecision{
		Decision:          decision,
		AnalyticsFields:   bundle.AnalyticsFields,
		ObjectId:          bundle.ObjectId,
		TriggerObjectType: bundle.Scenario.TriggerObjectType,
	}
}

// ToBundle returns a bundle carrying everything required to store the decision.
func (td TransientDecision) ToBundle() DecisionBundle {
	return DecisionBundle{
		Decision:        td.Decision,
		Scenario:        Scenario{TriggerObjectType: td.TriggerObjectType},
		AnalyticsFields: td.AnalyticsFields,
		ObjectId:        td.ObjectId,
	}
}

func (td TransientDecision) Encode() ([]byte, error) {
	b, err := json.Marshal(td)
	if err != nil {
		return nil, errors.Wrap(err, "could not serialize transient decision")
	}

	return transientDecisionEncoder.EncodeAll(b, make([]byte, 0, len(b)/8)), nil
}

func DecodeTransientDecision(payload []byte) (TransientDecision, error) {
	b, err := transientDecisionDecoder.DecodeAll(payload, nil)
	if err != nil {
		return TransientDecision{}, errors.Wrap(err, "could not decompress transient decision")
	}

	var td TransientDecision

	if err := json.Unmarshal(b, &td); err != nil {
		return TransientDecision{}, errors.Wrap(err, "could not deserialize transient decision")
	}

	return td, nil
}
