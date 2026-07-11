// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mdhender/tpty/internal/orders"
)

// locationOf returns the current stored location of the entity with the given id.
func locationOf(t *testing.T, s GameState, id int) string {
	t.Helper()
	e, ok := s.Entities.ByID(id)
	if !ok {
		t.Fatalf("entity %d not in store", id)
	}
	return e.Location
}

// completionTick scans a turn result's log for the tick on which the given
// entity completed an order and returns it, or -1 if none is found. The log
// line format is fixed by workEntity.
func completionTick(res TurnResult, entityID int) int {
	needle := fmt.Sprintf("entity %d completed", entityID)
	for _, line := range res.Log {
		if !strings.Contains(line, needle) {
			continue
		}
		var tick, id int
		if n, err := fmt.Sscanf(line, "tick %d: entity %d completed", &tick, &id); err == nil && n == 2 && id == entityID {
			return tick
		}
	}
	return -1
}

// TestHoldHandlerNonStubNoOp confirms Hold runs as a real, non-stub order: cost
// 7, an executed outcome (Stub=false), and no change to the entity's location.
func TestHoldHandlerNonStubNoOp(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	if got := (holdHandler{}).Cost(s, Entity{ID: 1}, mkOrder(orders.CmdHold, "hold"), 1, 1); got != 7 {
		t.Errorf("hold cost = %d, want 7", got)
	}

	res := ProcessTurn(TurnInput{
		State:     s,
		Turn:      1,
		Submitted: []StoredOrders{{Turn: 1, PlayerID: 1, Raw: "g 1 pw\nentity 1, Conan\n    hold\n"}},
	})

	if len(res.Outcomes) != 1 {
		t.Fatalf("outcomes = %d, want 1: %+v", len(res.Outcomes), res.Outcomes)
	}
	oc := res.Outcomes[0]
	if oc.Order.ID != orders.CmdHold {
		t.Errorf("outcome order = %v, want hold", oc.Order.Word)
	}
	if oc.Stub {
		t.Error("hold should now be a real handler, not a stub (Stub=true)")
	}
	if got := completionTick(res, 1); got != 7 {
		t.Errorf("hold completed on tick %d, want 7", got)
	}
	if loc := locationOf(t, s, 1); loc != "(0,0)" {
		t.Errorf("hold changed location to %q, want unchanged (0,0)", loc)
	}
	if len(res.Carryover) != 0 {
		t.Errorf("carryover = %+v, want none", res.Carryover)
	}
}

// TestMoveHandlerSingleStep confirms a one-direction move relocates the entity by
// one hex and completes at tick 7. From (0,0) a "move 2" (NE) lands at (1,-1),
// per content/docs/reference/world-generation.md.
func TestMoveHandlerSingleStep(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	if got := (moveHandler{}).Cost(s, Entity{Location: "(0,0)"}, orders.Order{ID: orders.CmdMove, Word: "move", Args: []string{"2"}}, 1, 1); got != 7 {
		t.Errorf("move (1 step) cost = %d, want 7", got)
	}

	res := ProcessTurn(TurnInput{
		State:     s,
		Turn:      1,
		Submitted: []StoredOrders{{Turn: 1, PlayerID: 1, Raw: "g 1 pw\nentity 1, Conan\n    move 2\n"}},
	})

	if len(res.Outcomes) != 1 || res.Outcomes[0].Stub {
		t.Fatalf("outcomes = %+v, want one non-stub move", res.Outcomes)
	}
	if got := completionTick(res, 1); got != 7 {
		t.Errorf("move completed on tick %d, want 7", got)
	}
	if loc := locationOf(t, s, 1); loc != "(1,-1)" {
		t.Errorf("after move 2 from (0,0), location = %q, want (1,-1)", loc)
	}
}

// TestMoveHandlerMultiStep confirms a multi-direction path lands at the correct
// final hex and costs 7 per step. From (0,0), "move 3 3 4" (SE, SE, S) lands at
// (2,1): (0,0)+(1,0)+(1,0)+(0,1).
func TestMoveHandlerMultiStep(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	o := orders.Order{ID: orders.CmdMove, Word: "move", Args: []string{"3", "3", "4"}}
	if got := (moveHandler{}).Cost(s, Entity{Location: "(0,0)"}, o, 1, 1); got != 21 {
		t.Errorf("move (3 steps) cost = %d, want 21", got)
	}

	res := ProcessTurn(TurnInput{
		State:     s,
		Turn:      1,
		Submitted: []StoredOrders{{Turn: 1, PlayerID: 1, Raw: "g 1 pw\nentity 1, Conan\n    move 3 3 4\n"}},
	})

	if len(res.Outcomes) != 1 || res.Outcomes[0].Stub {
		t.Fatalf("outcomes = %+v, want one non-stub move", res.Outcomes)
	}
	if got := completionTick(res, 1); got != 21 {
		t.Errorf("move (3 steps) completed on tick %d, want 21", got)
	}
	if loc := locationOf(t, s, 1); loc != "(2,1)" {
		t.Errorf("after move 3 3 4 from (0,0), location = %q, want (2,1)", loc)
	}
}

// TestMoveHandlerInvalidDirectionFails confirms an out-of-range direction number
// makes the move fail without moving: a non-stub failure outcome, cost 0, and an
// unchanged location.
func TestMoveHandlerInvalidDirectionFails(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	bad := orders.Order{ID: orders.CmdMove, Word: "move", Args: []string{"7"}}
	if got := (moveHandler{}).Cost(s, Entity{Location: "(0,0)"}, bad, 1, 1); got != 0 {
		t.Errorf("invalid move cost = %d, want 0", got)
	}

	res := ProcessTurn(TurnInput{
		State:     s,
		Turn:      1,
		Submitted: []StoredOrders{{Turn: 1, PlayerID: 1, Raw: "g 1 pw\nentity 1, Conan\n    move 7\n"}},
	})

	if len(res.Outcomes) != 1 {
		t.Fatalf("outcomes = %d, want 1: %+v", len(res.Outcomes), res.Outcomes)
	}
	oc := res.Outcomes[0]
	if oc.Stub {
		t.Error("an invalid move should fail as a real (non-stub) outcome")
	}
	if !strings.Contains(oc.Message, "invalid direction") {
		t.Errorf("failure message = %q, want it to mention the invalid direction", oc.Message)
	}
	if loc := locationOf(t, s, 1); loc != "(0,0)" {
		t.Errorf("invalid move changed location to %q, want unchanged (0,0)", loc)
	}
}

// TestMoveHandlerDeterministic confirms two identical runs of a move produce
// identical results and identical final entity locations.
func TestMoveHandlerDeterministic(t *testing.T) {
	run := func() (TurnResult, string) {
		s := newEngineState(t)
		createEntity(t, s, "Conan", "(0,0)")
		res := ProcessTurn(TurnInput{
			State:     s,
			Turn:      1,
			Submitted: []StoredOrders{{Turn: 1, PlayerID: 1, Raw: "g 1 pw\nentity 1, Conan\n    move 2 3 4\n"}},
		})
		return res, locationOf(t, s, 1)
	}

	res1, loc1 := run()
	res2, loc2 := run()

	if loc1 != loc2 {
		t.Errorf("final locations differ: %q vs %q", loc1, loc2)
	}
	// (0,0)+NE(1,-1)+SE(1,0)+S(0,1) = (2,0).
	if loc1 != "(2,0)" {
		t.Errorf("final location = %q, want (2,0)", loc1)
	}
	if len(res1.Outcomes) != len(res2.Outcomes) {
		t.Fatalf("outcome counts differ: %d vs %d", len(res1.Outcomes), len(res2.Outcomes))
	}
	if res1.Outcomes[0].Message != res2.Outcomes[0].Message {
		t.Errorf("outcome messages differ: %q vs %q", res1.Outcomes[0].Message, res2.Outcomes[0].Message)
	}
}

// TestSetLocationUnknownEntity confirms SetLocation rejects a missing id with
// ErrUnknownEntity and rejects a non-canonical province.
func TestSetLocationUnknownEntity(t *testing.T) {
	store := NewEntityStore()
	if _, err := store.Create("Conan", 1, "(0,0)"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.SetLocation(99, "(1,0)"); err == nil {
		t.Error("SetLocation(missing id) = nil, want ErrUnknownEntity")
	}
	if err := store.SetLocation(1, "(0, 0)"); err == nil {
		t.Error("SetLocation with non-canonical province = nil, want error")
	}

	if err := store.SetLocation(1, "(1,-1)"); err != nil {
		t.Fatalf("SetLocation valid: %v", err)
	}
	if e, _ := store.ByID(1); e.Location != "(1,-1)" {
		t.Errorf("location = %q, want (1,-1)", e.Location)
	}
}
