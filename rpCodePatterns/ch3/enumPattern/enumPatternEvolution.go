package main

import (
	"encoding/json"
	"fmt"
)

// =============================================================================
// PATTERN: Enum pattern with generics (Go)
// Evolution: naive consts -> per-type hand-rolled validation -> generic
// single-type Enum[T] -> generic Enum[T,V] supporting any comparable
// underlying type (string/int/float/...).
// =============================================================================

// -----------------------------------------------------------------------------
// STAGE 1: Naive version
// PROBLEM: represent a fixed set of allowed values (order status) and stop
// invalid values from being used.
// -----------------------------------------------------------------------------

type NaiveOrderStatus string

const (
	NaiveOrderPending   NaiveOrderStatus = "pending"
	NaiveOrderShipped   NaiveOrderStatus = "shipped"
	NaiveOrderDelivered NaiveOrderStatus = "delivered"
)

// LIMITATION: the underlying type is just `string`, so the compiler cannot
// stop anyone from constructing a value that was never declared above.
func processNaiveOrderStatus(s NaiveOrderStatus) {
	fmt.Println("processing order:", s)
}

func demoStage1() {
	processNaiveOrderStatus(NaiveOrderShipped) // fine

	// LIMITATION: typo compiles and runs silently wrong - no validation
	// exists anywhere, at compile time or runtime.
	processNaiveOrderStatus(NaiveOrderStatus("shpped"))

	// LIMITATION: the exact same problem hits data coming from JSON/DB -
	// there is no hook to reject "shpped" during unmarshaling either.
}

// -----------------------------------------------------------------------------
// STAGE 2: Intermediate step #1 - hand-rolled validation per type
// PROBLEM: stage 1 never rejects invalid values. Add an explicit allow-list
// and a validating Unmarshal so bad input from JSON/DB is caught early.
// -----------------------------------------------------------------------------

type PaymentStatus struct {
	value string
}

// FIX: Values() gives one explicit source of truth for what's allowed,
// closing the "typo compiles" LIMITATION from stage 1.
func (PaymentStatus) Values() []string {
	return []string{"authorized", "captured", "refunded"}
}

// FIX: UnmarshalText rejects anything not in Values() before it reaches
// application code - closes the "bad JSON/DB input" LIMITATION from stage 1.
func (p *PaymentStatus) UnmarshalText(text []byte) error {
	for _, v := range p.Values() {
		if v == string(text) {
			p.value = v
			return nil
		}
	}
	return fmt.Errorf("invalid payment status %q, expected %v", text, p.Values())
}

func (p PaymentStatus) String() string { return p.value }

// STILL BROKEN: adding another enum (e.g. shipping status) means copying
// this entire struct + three methods again - Values/UnmarshalText/String
// are re-implemented per type with zero code reuse.
// STILL BROKEN: `value string` locks every enum built this way to a string
// underlying type - an int-backed or float-backed enum needs a whole new
// hand-written copy of this block.

// -----------------------------------------------------------------------------
// STAGE 3: Intermediate step #2 - generic single-type-param Enum[T]
// PROBLEM: stage 2 duplicates Values/UnmarshalText/String for every new
// enum. Factor the reusable part (matching + storing) into one generic
// type, so each enum only has to supply its own Values().
// (This is the same idea as Enumerable/Enum/States already below in
// countryCode.go - repeated here, under different names, so the four
// stages read as one continuous story in a single file.)
// -----------------------------------------------------------------------------

// EnumValues is the only thing a new enum needs to implement now.
type EnumValues interface {
	Values() []string
}

// StrEnum is written exactly once and reused by every string-backed enum.
type StrEnum[T EnumValues] struct {
	value string
}

// FIX: this method exists once, not once per enum - closes the "duplicated
// boilerplate" STILL BROKEN item from stage 2.
func (e *StrEnum[T]) UnmarshalText(text []byte) error {
	var marker T
	for _, v := range marker.Values() {
		if v == string(text) {
			e.value = v
			return nil
		}
	}
	return fmt.Errorf("invalid value %q, expected %v", text, marker.Values())
}

func (e StrEnum[T]) String() string { return e.value }

// Each new enum now costs just a marker type + one Values() method.
type ShippingStatusKind struct{}

func (ShippingStatusKind) Values() []string {
	return []string{"pending", "in_transit", "delivered"}
}

type ShippingStatus struct {
	StrEnum[ShippingStatusKind]
}

// STILL BROKEN: `StrEnum[T]` still hard-codes `value string`. A price-tier
// enum backed by `int`, or a rating enum backed by `float64`, cannot use
// this type at all - the same type-lock-in problem as stage 2, just moved
// up one level instead of solved.

// -----------------------------------------------------------------------------
// STAGE 4: Final pattern - generic Enum[T, V] over any comparable value type
// PROBLEM: stage 3 only supports string-backed enums. Add a second type
// parameter for the underlying value type so int/float/string all reuse
// the exact same machinery.
// -----------------------------------------------------------------------------

// Enumerable2 is generic over V - Values() now returns whatever type the
// enum is actually backed by, not just string.
type Enumerable2[V comparable] interface {
	Values() []V
}

// Enum2 takes both the marker type T and its value type V. This is the
// mechanism that closes the type-lock-in LIMITATION: V is no longer fixed
// to `string` inside the generic type itself.
type Enum2[T Enumerable2[V], V comparable] struct {
	value V
}

// FIX: json.Unmarshal already knows how to decode raw bytes into int,
// float64, string, etc. - so parsing is delegated to it instead of being
// hand-written per type. This is what lets one generic type serve every
// underlying kind, closing the last STILL BROKEN item from stage 3.
func (e *Enum2[T, V]) UnmarshalJSON(data []byte) error {
	var v V
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	var marker T
	for _, allowed := range marker.Values() {
		if allowed == v {
			e.value = v
			return nil
		}
	}
	return fmt.Errorf("invalid value %v, expected %v", v, marker.Values())
}

func (e Enum2[T, V]) Value() V       { return e.value }
func (e Enum2[T, V]) String() string { return fmt.Sprint(e.value) }

// -- Usage: the exact same Enum2 machinery backs a string enum...
type PaymentStatusKind struct{}

func (PaymentStatusKind) Values() []string {
	return []string{"authorized", "captured", "refunded"}
}

type PaymentStatusV4 struct{ Enum2[PaymentStatusKind, string] }

// ...an int enum...
type PriorityKind struct{}

func (PriorityKind) Values() []int { return []int{1, 2, 3} }

type Priority struct{ Enum2[PriorityKind, int] }

// ...and a float enum, with zero new parsing/validation code written.
type RatingKind struct{}

func (RatingKind) Values() []float64 { return []float64{0.5, 1.0, 1.5} }

type Rating struct{ Enum2[RatingKind, float64] }

// -----------------------------------------------------------------------------
// WHEN NOT TO USE STAGE 4
// - A one-off enum with 2-3 values used in one place: stage 1 (plain
//   consts) is fine - the generic machinery is pure overhead for something
//   that never needs runtime validation.
// - If the type needs to implement `database/sql.Scanner` /
//   `driver.Valuer` or other concrete-type interfaces some libraries
//   expect, the extra type parameter can make those method signatures
//   awkward to satisfy - a hand-written stage-2-style type is sometimes
//   simpler there.
// - Reflection-based `json.Unmarshal` per value has a real (if small) cost;
//   in a hot path parsing millions of values, a hand-written per-type
//   parser (stage 2/3 style) can be faster.
// -----------------------------------------------------------------------------

// demoStage4 is left uncalled - main() already lives in countryCode.go.
// Call it from there manually if you want to see stage 4 run.
func demoStage4() {
	var p PaymentStatusV4
	_ = p.UnmarshalJSON([]byte(`"captured"`))
	fmt.Println(p)

	var pr Priority
	_ = pr.UnmarshalJSON([]byte(`2`))
	fmt.Println(pr)

	var r Rating
	_ = r.UnmarshalJSON([]byte(`1.0`))
	fmt.Println(r)
}
