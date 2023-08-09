package evaluate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWalkWindowFindFractionated(t *testing.T) {
	numberThreshold := 2
	amountThreshold := 1000.0

	// test valid return cases
	type testCase struct {
		transactions []map[string]any
		expected     bool
		name         string
	}
	testCases := []testCase{
		{transactions: []map[string]any{}, expected: false, name: "empty transactions"},
		{transactions: []map[string]any{
			{"counterparty_iban": "iban1", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
		}, expected: false, name: "no duplicate iban"},
		{transactions: []map[string]any{
			{"counterparty_iban": "iban1", "created_at": time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "iban1", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "iban1", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
		}, expected: true, name: "with fractionated iban"},
		{transactions: []map[string]any{
			{"counterparty_iban": "iban1", "created_at": time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC), "txn_amount": 10.0},
			{"counterparty_iban": "iban1", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 10.0},
			{"counterparty_iban": "iban1", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 10.0},
		}, expected: false, name: "with fractionated iban low amount"},
		{transactions: []map[string]any{
			{"counterparty_iban": "iban 1", "created_at": time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "iban 2", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "iban 1", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "iban 2", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
		}, expected: true, name: "with fractionated iban 2"},
	}
	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			found, err := walkWindowFindFractionated(c.transactions, numberThreshold, amountThreshold, 1)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, found)
		})
	}

	// Test no panic on missing fields
	transactions := []map[string]any{
		{"txn_amount": 1000.0},
	}
	_, err := walkWindowFindFractionated(transactions, numberThreshold, amountThreshold, 1)
	assert.Error(t, err)
}

func TestWalkWindowFindMultipleNonFrTransfers(t *testing.T) {
	numberThreshold := 2
	amountThreshold := 1000.0

	// test valid return cases
	type testCase struct {
		transactions []map[string]any
		expected     bool
		name         string
	}
	testCases := []testCase{
		{transactions: []map[string]any{}, expected: false, name: "empty transactions"},
		{transactions: []map[string]any{
			{"counterparty_iban": "FR1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
		}, expected: false, name: "only fr iban"},
		{transactions: []map[string]any{
			{"counterparty_iban": "FR1234", "created_at": time.Date(2020, 1, 2, 12, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
		}, expected: false, name: "series beginning with FR transaction evaluates to false because the first tx is the reference"},
		{transactions: []map[string]any{
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
		}, expected: true, name: "with 2 non fr transfers, high amount"},
		{transactions: []map[string]any{
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 2000.0},
		}, expected: false, name: "Only 1 non fr transfers, high amount"},
		{transactions: []map[string]any{
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC), "txn_amount": 10.0},
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 10.0},
			{"counterparty_iban": "FR1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 2000.0},
		}, expected: false, name: "with 2 non fr transfers, low amount"},
		{transactions: []map[string]any{
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "FR1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
			{"counterparty_iban": "LT1234", "created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "txn_amount": 1000.0},
		}, expected: true, name: "with 2 non fr transfers, high amount 2"},
	}
	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			found, err := walkWindowFindMultipleNonFrTransfers(c.transactions, numberThreshold, amountThreshold, 2)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, found)
		})
	}

	// Test no panic on missing fields
	transactions := []map[string]any{
		{"txn_amount": 1000.0},
	}
	_, err := walkWindowFindMultipleNonFrTransfers(transactions, numberThreshold, amountThreshold, 2)
	assert.Error(t, err)
}
