package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"marble/marble-backend/models"
)

// ///////////////////////////////////////////////////////////////////////////////////////
// Variable: Sum transactions (cash, payout, one month) for Julaya
// ///////////////////////////////////////////////////////////////////////////////////////

// To be moved to the data accessor to be able to check at runtime if the scenario belongs to the proper org
func getOrgName() string {
	return "Julaya"
}

type JulayaSumCashPayoutOneMonth struct{}

// register creation
func init() {
	operatorFromType["JULAYA_SUM_CASH_PAYOUT_ONE_MONTH"] = func() Operator { return &JulayaSumCashPayoutOneMonth{} }
}

func (r JulayaSumCashPayoutOneMonth) Eval(ctx context.Context, d DataAccessor) (float64, error) {
	db := d.GetDbHandle()

	// Get account_id from payload. Basically a copy/paste from the string payload field operator Eval()
	accountIdRaw, err := d.GetPayloadField("account_id")
	if err != nil {
		return 0, err
	}

	accountIdPointer, ok := accountIdRaw.(*string)
	if !ok {
		return 0, fmt.Errorf("Payload field %s is not a pointer to a string", "account_id")
	}
	if accountIdPointer == nil {
		return 0, fmt.Errorf("Payload field %s is null: %w", "account_id", models.OperatorNullValueReadError)
	}

	// Execute query with the account id
	sql := `
		SELECT SUM(amount)
		FROM transactions
		WHERE account_id = $1
			AND type='CASH')
			AND status='VALIDATED'
			AND direction='PAYOUT'
			AND transaction_at > NOW() - INTERVAL '1 MONTH'
			AND transaction_at < NOW()
	`
	// NB: in this implementation, nothing is stopping us from also retrieving the transaction_date from the
	// payload, instead of using SQL NOW().
	// Here, I'm skipping on this to keep it simple and keep the two implementations functionally equivalent.
	args := []any{*accountIdPointer}
	rows := db.QueryRow(ctx, sql, args...)
	var output float64
	err = rows.Scan(&output)
	if err != nil {
		return 0, err
	}

	return output, nil
}

func (r JulayaSumCashPayoutOneMonth) IsValid() bool {
	return getOrgName() == "Julaya"
}

func (r JulayaSumCashPayoutOneMonth) String() string {
	return "Julaya variable: sum cash payouts one month prior to trigger"
}

func (r JulayaSumCashPayoutOneMonth) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		OperatorType
	}{
		OperatorType: OperatorType{Type: "JULAYA_SUM_CASH_PAYOUT_ONE_MONTH"},
	})
}

func (r *JulayaSumCashPayoutOneMonth) UnmarshalJSON(b []byte) error {
	return nil
}
