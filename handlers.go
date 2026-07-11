// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
	"strconv"

	"github.com/mdhender/tpty/internal/orders"
)

// holdHandler executes the Hold command (id 0): the explicit "do nothing this
// week" order. It costs a fixed 7 ticks and changes no world state, recording an
// executed (non-stub) no-op. It draws no randomness.
//
// See content/docs/reference/orders/hold.md.
type holdHandler struct{}

// holdCost is Hold's fixed time cost in ticks — 7 days, one week — matching the
// command summary in content/docs/reference/orders/_index.md.
const holdCost = 7

// Cost reports Hold's fixed cost of 7 ticks.
func (holdHandler) Cost(_ GameState, _ Entity, _ orders.Order, _, _ int) int {
	return holdCost
}

// Apply records that the entity held: a completed, non-stub order that changes
// nothing.
func (holdHandler) Apply(_ GameState, e Entity, o orders.Order, _, _ int) OrderOutcome {
	return OrderOutcome{
		EntityID: e.ID,
		Order:    o,
		Stub:     false,
		Message:  fmt.Sprintf("%s: held (no action taken)", o.Word),
	}
}

// moveStepCost is the time cost, in ticks, of one Move step — one adjacent hex
// at 7 days (one week). Total Move cost is moveStepCost times the number of
// steps in the path. See content/docs/reference/orders/move.md.
const moveStepCost = 7

// moveHandler executes the Move command (id 1): it walks the entity along a path
// of direction numbers (1..6, clockwise from north) and updates its location. It
// draws no randomness.
//
// See content/docs/reference/orders/move.md and the direction vectors in
// content/docs/reference/world-generation.md.
type moveHandler struct{}

// moveDirVector maps a direction number (1..6, clockwise from north) to its
// axial step vector, matching the six neighbor directions in
// content/docs/reference/world-generation.md. The second result is false for a
// number outside 1..6.
func moveDirVector(n int) (Hex, bool) {
	switch n {
	case 1:
		return DirN, true
	case 2:
		return DirNE, true
	case 3:
		return DirSE, true
	case 4:
		return DirS, true
	case 5:
		return DirSW, true
	case 6:
		return DirNW, true
	default:
		return Hex{}, false
	}
}

// parseMovePath interprets a Move order's arguments as a path of direction
// numbers and returns the corresponding step vectors. The ok result is false if
// any argument is not an integer in 1..6, in which case badArg names the
// offending argument (for the failure message) and steps is nil. An empty path
// is also invalid — the parser guarantees at least one argument, so this only
// guards a malformed queue entry.
func parseMovePath(o orders.Order) (steps []Hex, badArg string, ok bool) {
	if len(o.Args) == 0 {
		return nil, "", false
	}
	steps = make([]Hex, 0, len(o.Args))
	for _, arg := range o.Args {
		n, err := strconv.Atoi(arg)
		if err != nil {
			return nil, arg, false
		}
		v, ok := moveDirVector(n)
		if !ok {
			return nil, arg, false
		}
		steps = append(steps, v)
	}
	return steps, "", true
}

// Cost reports the Move's time cost: moveStepCost per step for a valid path.
// A malformed path (a direction argument that is not an integer 1..6) is treated
// as a zero-time failure, consistent with Apply, so it completes immediately and
// Apply records the failure.
func (moveHandler) Cost(_ GameState, _ Entity, o orders.Order, _, _ int) int {
	steps, _, ok := parseMovePath(o)
	if !ok {
		return 0
	}
	return moveStepCost * len(steps)
}

// Apply moves the entity along the parsed path and updates its location in the
// store, returning a completed (non-stub) outcome describing the move from its
// starting province to its destination. A malformed path fails: the entity does
// not move and the outcome names the bad argument. Apply performs no I/O and
// draws no randomness.
func (moveHandler) Apply(s GameState, e Entity, o orders.Order, _, _ int) OrderOutcome {
	steps, badArg, ok := parseMovePath(o)
	if !ok {
		return OrderOutcome{
			EntityID: e.ID,
			Order:    o,
			Stub:     false,
			Message:  fmt.Sprintf("%s: invalid direction %q — must be 1..6; entity did not move", o.Word, badArg),
		}
	}

	from, err := canonicalProvince(e.Location)
	if err != nil {
		return OrderOutcome{
			EntityID: e.ID,
			Order:    o,
			Stub:     false,
			Message:  fmt.Sprintf("%s: entity has non-canonical location %q; entity did not move", o.Word, e.Location),
		}
	}

	dest := from
	for _, step := range steps {
		dest = dest.Add(step)
	}

	if err := s.Entities.SetLocation(e.ID, dest.String()); err != nil {
		return OrderOutcome{
			EntityID: e.ID,
			Order:    o,
			Stub:     false,
			Message:  fmt.Sprintf("%s: could not update location: %v", o.Word, err),
		}
	}

	return OrderOutcome{
		EntityID: e.ID,
		Order:    o,
		Stub:     false,
		Message:  fmt.Sprintf("%s: moved from %s to %s", o.Word, from.String(), dest.String()),
	}
}
