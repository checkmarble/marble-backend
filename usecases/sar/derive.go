package sar

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

// Backend derivations. Everything computable is computed here and never
// asked of the model: conversions, sums, counts, periods and date formats.

// Convert converts an exact amount with an explicit configured rate
// ("FROM/TO" -> rate). It never infers a rate.
func Convert(amount, from, to string, rates map[string]string) (string, string, bool) {
	a, ok := new(big.Rat).SetString(amount)
	if !ok {
		return "", "", false
	}
	if from == to {
		return Round(a, 2), "", true
	}
	rate, ok := rates[from+"/"+to]
	if !ok {
		if inverse, ok2 := rates[to+"/"+from]; ok2 {
			r, ok3 := new(big.Rat).SetString(inverse)
			if !ok3 || r.Sign() == 0 {
				return "", "", false
			}
			res := new(big.Rat).Quo(a, r)
			return Round(res, 2), fmt.Sprintf("%s %s / %s (taux %s/%s configuré)", amount, from, inverse, to, from), true
		}
		return "", "", false
	}
	r, ok := new(big.Rat).SetString(rate)
	if !ok {
		return "", "", false
	}
	res := new(big.Rat).Mul(a, r)
	return Round(res, 2), fmt.Sprintf("%s %s × %s (taux %s/%s configuré)", amount, from, rate, from, to), true
}

// Round formats a rational with half-up rounding to n decimals.
func Round(r *big.Rat, n int) string {
	s := new(big.Rat).Abs(r)
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	num := new(big.Int).Mul(s.Num(), scale)
	q, rem := new(big.Int).QuoRem(num, s.Denom(), new(big.Int))
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(s.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	str := q.String()
	for len(str) <= n {
		str = "0" + str
	}
	out := str[:len(str)-n]
	if n > 0 {
		out += "." + str[len(str)-n:]
	}
	if r.Sign() < 0 && strings.Trim(out, "0.") != "" {
		out = "-" + out
	}
	return out
}

// Sum adds exact decimals.
func Sum(values []string) (string, bool) {
	total := new(big.Rat)
	for _, v := range values {
		r, ok := new(big.Rat).SetString(v)
		if !ok {
			return "", false
		}
		total.Add(total, r)
	}
	return Round(total, 2), true
}

// Abs returns the absolute value of an exact decimal.
func Abs(v string) string { return strings.TrimPrefix(v, "-") }

var Paris = mustLocation("Europe/Paris")

func mustLocation(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return l
}

// LocalDate returns the civil date of a timestamp in a timezone.
func LocalDate(ts string, loc *time.Location) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		if d, err2 := time.Parse("2006-01-02", ts); err2 == nil {
			return d, true
		}
		return time.Time{}, false
	}
	return t.In(loc), true
}

// FrenchDate converts an ISO date or timestamp to dd/mm/yyyy (Paris time).
func FrenchDate(v string) string {
	t, ok := LocalDate(v, Paris)
	if !ok {
		return ""
	}
	return t.Format("02/01/2006")
}

// GoAMLDateTime converts an ISO date or timestamp to the goAML local
// xs:dateTime without offset, in the FIU timezone.
func GoAMLDateTime(v string, loc *time.Location) string {
	if d, err := time.Parse("2006-01-02", v); err == nil {
		return d.Format("2006-01-02") + "T00:00:00"
	}
	t, ok := LocalDate(v, loc)
	if !ok {
		return ""
	}
	return t.Format("2006-01-02T15:04:05")
}

// Period returns the earliest and latest operation instants.
func Period(ops []*models.SarOperation) (time.Time, time.Time, bool) {
	var first, last time.Time
	ok := false
	for _, op := range ops {
		t, err := time.Parse(time.RFC3339, op.Timestamp.Val())
		if err != nil {
			continue
		}
		if !ok || t.Before(first) {
			first = t
		}
		if !ok || t.After(last) {
			last = t
		}
		ok = true
	}
	return first, last, ok
}

// Calc is the provenance of a backend computation.
func Calc(explanation string, sources ...models.SarSourceRef) *models.SarProvenance {
	return &models.SarProvenance{Kind: models.SarProvCalculated, Explanation: explanation, Sources: sources}
}
