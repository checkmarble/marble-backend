package operators

import (
	"context"
	"encoding/json"
)

func checkOrgIdJulaya(orgID string) bool {
	// TODO adapt once org provisioned
	return orgID == "JULAYA_ORG_ID"
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Variable: Sum transactions (cash, payout, one month) for Julaya
// ///////////////////////////////////////////////////////////////////////////////////////

type JulayaSumCashPayoutOneMonth struct{}

// register creation
func init() {
	operatorFromType["JULAYA_SUM_CASH_PAYOUT_ONE_MONTH"] = func() Operator { return &JulayaSumCashPayoutOneMonth{} }
}

func (r JulayaSumCashPayoutOneMonth) Eval(ctx context.Context, d DataAccessor) (float64, error) {
	db := d.GetDbHandle()

	accountId, err := getPayloadFieldGeneric[string](d, "account_id")
	if err != nil {
		return 0, err
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
	args := []any{accountId}
	return queryDbFieldGeneric[float64](ctx, db, sql, args)
}

func (r JulayaSumCashPayoutOneMonth) IsValid() bool {
	return true
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
