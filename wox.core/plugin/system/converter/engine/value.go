// Package engine implements converter syntax and evaluation without plugin or network access.
package engine

import (
	"fmt"
	"math/big"
	"time"
)

type ErrorKind string

const (
	Unrecognized ErrorKind = "unrecognized"
	Invalid      ErrorKind = "invalid"
	Unavailable  ErrorKind = "unavailable"
)

type Error struct {
	Kind     ErrorKind
	Position int
	Message  string
}

func (e *Error) Error() string     { return fmt.Sprintf("%s at %d: %s", e.Kind, e.Position, e.Message) }
func invalid(message string) error { return &Error{Kind: Invalid, Message: message} }

type ParseOptions struct{ ThousandsSeparator, DecimalSeparator string }

// Env is immutable for one evaluation. Prices are USD per unit, including fiat.
type Env struct {
	Now             time.Time
	Local           *time.Location
	DefaultCurrency string
	Prices          map[string]*big.Rat
	RateUpdatedAt   int64
	FPS             *big.Rat
	Substance       string
}

type Kind uint8

const (
	Number Kind = iota
	Percent
	Quantity
	Date
	Clock
	Instant
	CalendarSpan
	Boolean
)

func isTemporal(k Kind) bool {
	return k == Date || k == Clock || k == Instant || k == CalendarSpan
}

// Unit retains authored factors for display; dimensions are derived from definitions.
type Unit map[string]int

// Value separates numerical meaning from display. Months/days are calendar spans,
// while seconds are quantities and instants retain a real timezone.
type Value struct {
	Kind         Kind
	Number       *big.Rat
	Unit         Unit
	Time         time.Time
	Months, Days int
	// Ampm is set when a clock literal used am/pm. Soulver clock-clock minus
	// then returns the absolute same-day interval; 24-hour clocks stay signed.
	Ampm bool
}

type Evaluation struct {
	Value         Value
	Source        *time.Time
	Target        string
	Expression    string
	Currency      bool
	RateUpdatedAt int64
	Ratio         string
	Decimals      *int
	Nearest       *big.Rat
	RoundDir      string
	FormatUnits   []string
	FPS           *big.Rat
	Substance     string
}

type Presentation struct {
	Formatted, Raw, Expression, SubTitle string
	TimeZone                             string
	Currency                             bool
	RateUpdatedAt                        int64
}

const maxBits = 16384

// checked bounds rational storage before a result can feed another operation.
func checked(v Value) (Value, error) {
	if v.Number != nil && (v.Number.Num().BitLen() > maxBits || v.Number.Denom().BitLen() > maxBits) {
		return Value{}, invalid("numeric result exceeds limit")
	}
	for _, exponent := range v.Unit {
		if exponent < -16 || exponent > 16 {
			return Value{}, invalid("unit exponent exceeds limit")
		}
	}
	return v, nil
}

func rational(s string) *big.Rat { r, _ := new(big.Rat).SetString(s); return r }
func number(n int64) Value       { return Value{Kind: Number, Number: big.NewRat(n, 1)} }
func copyUnit(u Unit) Unit {
	result := Unit{}
	for k, v := range u {
		result[k] = v
	}
	return result
}
