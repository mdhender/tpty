// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"reflect"
	"testing"

	"github.com/mdhender/tpty/internal/orders"
	"github.com/mdhender/tpty/internal/prng"
)

// newEngineState builds a GameState with an empty faction store and an entity
// store populated by createEntity. Locations are given verbatim and must be
// canonical.
func newEngineState(t *testing.T) GameState {
	t.Helper()
	g, err := NewGame("engine-test", prng.Seeds{Seed1: 1, Seed2: 2})
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	return GameState{Game: g, Entities: NewEntityStore(), Factions: NewFactionStore()}
}

// createEntity adds an entity at the given location and returns it. All test
// entities share faction 1; the engine's stub handlers do not consult factions.
func createEntity(t *testing.T, s GameState, name, location string) Entity {
	t.Helper()
	e, err := s.Entities.Create(name, 1, location)
	if err != nil {
		t.Fatalf("Create(%q, %q): %v", name, location, err)
	}
	return e
}

// mkOrder builds a parsed order with the given id and word (arguments empty).
func mkOrder(id orders.CommandID, word string) orders.Order {
	return orders.Order{ID: id, Word: word}
}

// recorded is one Apply call captured by a recordingHandler.
type recorded struct {
	entityID int
	cmd      orders.CommandID
	tick     int
}

// recordingHandler reports a fixed cost and records every Apply call, so tests
// can observe completion timing and the scheduler's resolution order.
type recordingHandler struct {
	cost int
	log  *[]recorded
}

func (h *recordingHandler) Cost(_ GameState, _ Entity, _ orders.Order, _, _ int) int {
	return h.cost
}

func (h *recordingHandler) Apply(_ GameState, e Entity, o orders.Order, _, tick int) OrderOutcome {
	*h.log = append(*h.log, recorded{entityID: e.ID, cmd: o.ID, tick: tick})
	return OrderOutcome{EntityID: e.ID, Order: o, Message: "recorded"}
}

// TestProcessTurnStubDispatch runs a submitted order end to end through
// ProcessTurn with an explicitly all-stub dispatch (newStubDispatch) and confirms
// it completes, records a stub no-op, and leaves no carryover. It uses the stub
// dispatch so it exercises the timeline mechanics of an unimplemented command
// regardless of which commands now have real handlers.
func TestProcessTurnStubDispatch(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	in := TurnInput{
		State:     s,
		Turn:      1,
		Dispatch:  newStubDispatch(),
		Submitted: []StoredOrders{{Turn: 1, PlayerID: 1, Raw: "g 1 pw\nentity 1, Conan\n    explore\n"}},
	}
	res := ProcessTurn(in)

	if len(res.Outcomes) != 1 {
		t.Fatalf("outcomes = %d, want 1: %+v", len(res.Outcomes), res.Outcomes)
	}
	oc := res.Outcomes[0]
	if oc.EntityID != 1 || oc.Order.ID != orders.CmdExplore {
		t.Errorf("outcome = %+v, want entity 1 explore", oc)
	}
	if !oc.Stub {
		t.Error("explore should be handled by the stub no-op (Stub=true)")
	}
	if oc.Message == "" {
		t.Error("stub outcome should carry a message")
	}
	if len(res.Carryover) != 0 {
		t.Errorf("carryover = %+v, want none (explore costs 7 < 30 ticks)", res.Carryover)
	}
}

// TestProcessTurnCompletionTiming confirms the ticks-left countdown: a cost-7
// order completes on tick 7, exercised through the full ProcessTurn path with a
// recording handler.
func TestProcessTurnCompletionTiming(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	var log []recorded
	d := NewDispatch().Register(orders.CmdHold, &recordingHandler{cost: 7, log: &log})
	res := ProcessTurn(TurnInput{
		State:     s,
		Turn:      1,
		Dispatch:  d,
		Carryover: []EntityQueue{{EntityID: 1, Orders: []orders.Order{mkOrder(orders.CmdHold, "hold")}}},
	})

	if len(log) != 1 {
		t.Fatalf("apply calls = %d, want 1: %+v", len(log), log)
	}
	if log[0].tick != 7 {
		t.Errorf("hold (cost 7) completed on tick %d, want 7", log[0].tick)
	}
	if len(res.Carryover) != 0 {
		t.Errorf("carryover = %+v, want none", res.Carryover)
	}
}

// TestProcessTurnZeroTimeConsumesNoWorkingTick confirms a zero-time order
// completes in the tick it becomes active and does not delay the order behind
// it: a [zero-time, cost-7] queue finishes the cost-7 order on tick 7, exactly
// as if the zero-time order were absent.
func TestProcessTurnZeroTimeConsumesNoWorkingTick(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	var log []recorded
	d := NewDispatch().
		Register(orders.CmdDrop, &recordingHandler{cost: 0, log: &log}).
		Register(orders.CmdHold, &recordingHandler{cost: 7, log: &log})
	ProcessTurn(TurnInput{
		State:    s,
		Turn:     1,
		Dispatch: d,
		Carryover: []EntityQueue{{EntityID: 1, Orders: []orders.Order{
			mkOrder(orders.CmdDrop, "drop"),
			mkOrder(orders.CmdHold, "hold"),
		}}},
	})

	if len(log) != 2 {
		t.Fatalf("apply calls = %d, want 2: %+v", len(log), log)
	}
	if log[0].cmd != orders.CmdDrop || log[0].tick != 1 {
		t.Errorf("drop completed on tick %d, want 1 (the tick it becomes active)", log[0].tick)
	}
	if log[1].cmd != orders.CmdHold || log[1].tick != 7 {
		t.Errorf("hold completed on tick %d, want 7 (zero-time drop consumed no working tick)", log[1].tick)
	}
}

// TestBuildQueuesCarryoverBeforeSubmitted confirms queue construction is FIFO
// with carryover ahead of newly submitted orders.
func TestBuildQueuesCarryoverBeforeSubmitted(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	in := TurnInput{
		State:     s,
		Turn:      2,
		Carryover: []EntityQueue{{EntityID: 1, Orders: []orders.Order{mkOrder(orders.CmdHold, "hold")}, Active: true, TicksLeft: 3}},
		Submitted: []StoredOrders{{Turn: 2, PlayerID: 1, Raw: "g 1 pw\nentity 1, Conan\n    explore\n"}},
	}
	queues := buildQueues(in, func(string, ...any) {})

	q, ok := queues[1]
	if !ok {
		t.Fatal("no queue for entity 1")
	}
	if len(q.orders) != 2 {
		t.Fatalf("queue length = %d, want 2: %+v", len(q.orders), q.orders)
	}
	if q.orders[0].ID != orders.CmdHold || q.orders[1].ID != orders.CmdExplore {
		t.Errorf("queue = [%v %v], want [hold explore]", q.orders[0].Word, q.orders[1].Word)
	}
	// The carried-in front order keeps its activation and remaining ticks.
	if !q.active || q.ticksLeft != 3 {
		t.Errorf("front order: active=%v ticksLeft=%d, want active=true ticksLeft=3", q.active, q.ticksLeft)
	}
}

// TestProcessTurnCarryoverFinishesBeforeSubmitted confirms a carried-over order
// completes before a newly submitted one begins.
func TestProcessTurnCarryoverFinishesBeforeSubmitted(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	res := ProcessTurn(TurnInput{
		State:     s,
		Turn:      2,
		Carryover: []EntityQueue{{EntityID: 1, Orders: []orders.Order{mkOrder(orders.CmdHold, "hold")}}},
		Submitted: []StoredOrders{{Turn: 2, PlayerID: 1, Raw: "g 1 pw\nentity 1, Conan\n    explore\n"}},
	})

	if len(res.Outcomes) != 2 {
		t.Fatalf("outcomes = %d, want 2: %+v", len(res.Outcomes), res.Outcomes)
	}
	if res.Outcomes[0].Order.ID != orders.CmdHold {
		t.Errorf("first completed order = %v, want hold (carryover finishes first)", res.Outcomes[0].Order.Word)
	}
	if res.Outcomes[1].Order.ID != orders.CmdExplore {
		t.Errorf("second completed order = %v, want explore", res.Outcomes[1].Order.Word)
	}
}

// TestProcessTurnCarryoverResumesWithRemainingTicks confirms an order that does
// not finish in one turn is carried over with its remaining ticks and completes
// on the next turn. Two follow orders (cost 28 each) exceed the 30 working
// ticks: the first completes on tick 28; the second is worked on ticks 29 and 30
// and carries 26 ticks into the next turn, where it completes.
func TestProcessTurnCarryoverResumesWithRemainingTicks(t *testing.T) {
	s := newEngineState(t)
	createEntity(t, s, "Conan", "(0,0)")

	turn1 := ProcessTurn(TurnInput{
		State:     s,
		Turn:      1,
		Carryover: []EntityQueue{{EntityID: 1, Orders: []orders.Order{mkOrder(orders.CmdFollow, "follow"), mkOrder(orders.CmdFollow, "follow")}}},
	})

	if len(turn1.Outcomes) != 1 || turn1.Outcomes[0].Order.ID != orders.CmdFollow {
		t.Fatalf("turn 1 outcomes = %+v, want one completed follow", turn1.Outcomes)
	}
	if len(turn1.Carryover) != 1 {
		t.Fatalf("turn 1 carryover = %+v, want one queue", turn1.Carryover)
	}
	cq := turn1.Carryover[0]
	if len(cq.Orders) != 1 || cq.Orders[0].ID != orders.CmdFollow {
		t.Errorf("carryover orders = %+v, want one follow", cq.Orders)
	}
	if !cq.Active || cq.TicksLeft != 26 {
		t.Errorf("carryover front: active=%v ticksLeft=%d, want active=true ticksLeft=26", cq.Active, cq.TicksLeft)
	}

	// Feed the carryover into turn 2; it resumes and completes.
	turn2 := ProcessTurn(TurnInput{State: s, Turn: 2, Carryover: turn1.Carryover})
	if len(turn2.Outcomes) != 1 || turn2.Outcomes[0].Order.ID != orders.CmdFollow {
		t.Fatalf("turn 2 outcomes = %+v, want the resumed follow completing", turn2.Outcomes)
	}
	if len(turn2.Carryover) != 0 {
		t.Errorf("turn 2 carryover = %+v, want none", turn2.Carryover)
	}
}

// TestSchedulerTotalOrdering confirms a working tick resolves active orders in
// one total order: priority ascending, then location q ascending, then r
// ascending, then seniority (stable store order within a location). Every order
// is given a zero cost so all resolve in tick 1, letting the outcome order
// reveal the scheduler order.
func TestSchedulerTotalOrdering(t *testing.T) {
	s := newEngineState(t)
	// Store order fixes seniority; ids are assigned 1..5 in this order.
	createEntity(t, s, "E1", "(0,0)") // hold  -> priority 4
	createEntity(t, s, "E2", "(5,5)") // move  -> priority 3
	createEntity(t, s, "E3", "(9,9)") // drop  -> priority 2
	createEntity(t, s, "E4", "(0,0)") // move  -> priority 3
	createEntity(t, s, "E5", "(0,0)") // hold  -> priority 4

	var log []recorded
	rec := &recordingHandler{cost: 0, log: &log}
	d := NewDispatch().
		Register(orders.CmdHold, rec).
		Register(orders.CmdMove, rec).
		Register(orders.CmdDrop, rec)

	in := TurnInput{
		State:    s,
		Turn:     1,
		Dispatch: d,
		Carryover: []EntityQueue{
			{EntityID: 1, Orders: []orders.Order{mkOrder(orders.CmdHold, "hold")}},
			{EntityID: 2, Orders: []orders.Order{mkOrder(orders.CmdMove, "move")}},
			{EntityID: 3, Orders: []orders.Order{mkOrder(orders.CmdDrop, "drop")}},
			{EntityID: 4, Orders: []orders.Order{mkOrder(orders.CmdMove, "move")}},
			{EntityID: 5, Orders: []orders.Order{mkOrder(orders.CmdHold, "hold")}},
		},
	}
	ProcessTurn(in)

	// Expected: priority 2 (E3), then priority 3 by q (E4 at q=0, then E2 at
	// q=5), then priority 4 at the shared (0,0) by seniority (E1 before E5).
	want := []int{3, 4, 2, 1, 5}
	if len(log) != len(want) {
		t.Fatalf("resolved %d orders, want %d: %+v", len(log), len(want), log)
	}
	for i, w := range want {
		if log[i].entityID != w {
			t.Errorf("resolution[%d] = entity %d, want %d (full order %v)", i, log[i].entityID, w, entityOrder(log))
		}
		if log[i].tick != 1 {
			t.Errorf("entity %d resolved on tick %d, want 1", log[i].entityID, log[i].tick)
		}
	}
}

// entityOrder extracts the entity ids from a recorded log, for error messages.
func entityOrder(log []recorded) []int {
	out := make([]int, len(log))
	for i, r := range log {
		out[i] = r.entityID
	}
	return out
}

// TestProcessTurnDeterministic confirms identical inputs produce an identical
// result: two runs of a multi-entity, multi-location turn are deeply equal.
func TestProcessTurnDeterministic(t *testing.T) {
	build := func() TurnInput {
		s := newEngineState(t)
		createEntity(t, s, "E1", "(0,0)")
		createEntity(t, s, "E2", "(1,0)")
		createEntity(t, s, "E3", "(0,1)")
		return TurnInput{
			State: s,
			Turn:  3,
			Submitted: []StoredOrders{
				{Turn: 3, PlayerID: 1, Raw: "g 1 pw\nentity 1, E1\n    hold\n    explore\n"},
				{Turn: 3, PlayerID: 2, Raw: "g 2 pw\nentity 2, E2\n    follow\n"},
				{Turn: 3, PlayerID: 3, Raw: "g 3 pw\nentity 3, E3\n    move 1\n"},
			},
		}
	}

	a := ProcessTurn(build())
	b := ProcessTurn(build())
	if !reflect.DeepEqual(a, b) {
		t.Errorf("ProcessTurn is not deterministic:\n a = %+v\n b = %+v", a, b)
	}
}

// TestCommandMetadataPriorities confirms the priority assignments from the
// turn-processing category table for a representative command in each category.
func TestCommandMetadataPriorities(t *testing.T) {
	cases := []struct {
		id       orders.CommandID
		priority int
		cost     int
	}{
		{orders.CmdMove, 3, stubVariesCost},   // move -> priority 3
		{orders.CmdWait, 2, stubVariesCost},   // wait -> priority 2
		{orders.CmdDrop, 2, 0},                // zero-time -> priority 2
		{orders.CmdGarrison, 2, 0},            // zero-time -> priority 2
		{orders.CmdHold, 4, 7},                // all-other -> priority 4
		{orders.CmdFollow, 4, 28},             // all-other -> priority 4
		{orders.CmdRecruit, 4, 14},            // all-other -> priority 4
		{orders.CmdTell, 2, 0},                // "0+" zero base -> priority 2
	}
	for _, c := range cases {
		m, ok := commandMetaFor(c.id)
		if !ok {
			t.Errorf("command %d: no metadata", c.id)
			continue
		}
		if m.priority != c.priority {
			t.Errorf("command %d priority = %d, want %d", c.id, m.priority, c.priority)
		}
		if m.cost != c.cost {
			t.Errorf("command %d cost = %d, want %d", c.id, m.cost, c.cost)
		}
		if m.priority < 1 || m.priority > 5 {
			t.Errorf("command %d priority %d is outside 1..5 (0 is the defect sentinel)", c.id, m.priority)
		}
	}
}

// TestCommandMetadataCoversAllCommands confirms every canonical command id
// (0..29, excluding the unused 7, 13, 17, 22) has metadata with a valid
// priority, so no parsed order reaches the scheduler at the priority-0 defect
// sentinel.
func TestCommandMetadataCoversAllCommands(t *testing.T) {
	all := []orders.CommandID{
		orders.CmdHold, orders.CmdMove, orders.CmdAttack, orders.CmdUse, orders.CmdTake,
		orders.CmdDrop, orders.CmdJoin, orders.CmdStudy, orders.CmdWork, orders.CmdBuy,
		orders.CmdSell, orders.CmdFollow, orders.CmdExplore, orders.CmdPersuade, orders.CmdSwear,
		orders.CmdPay, orders.CmdDeclare, orders.CmdRecruit, orders.CmdForm, orders.CmdPillage,
		orders.CmdExecute, orders.CmdTerrorize, orders.CmdWait, orders.CmdArmor, orders.CmdTell,
		orders.CmdGarrison,
	}
	for _, id := range all {
		m, ok := commandMetaFor(id)
		if !ok {
			t.Errorf("command %d has no metadata", id)
			continue
		}
		if m.priority < 1 || m.priority > 5 {
			t.Errorf("command %d priority = %d, want 1..5", id, m.priority)
		}
		if m.cost < 0 {
			t.Errorf("command %d cost = %d, want >= 0", id, m.cost)
		}
	}
}

// TestDispatchDefaultsToStub confirms an unregistered command id resolves to the
// stub no-op handler, and that Register binds a command to its handler. It uses
// newStubDispatch (no real handlers) so the fallback behaviour is tested
// independently of which commands NewDispatch now wires to real handlers.
func TestDispatchDefaultsToStub(t *testing.T) {
	d := newStubDispatch()
	if _, ok := d.handlerFor(orders.CmdHold).(stubHandler); !ok {
		t.Errorf("unregistered command routed to %T, want stubHandler", d.handlerFor(orders.CmdHold))
	}
	real := &recordingHandler{}
	d.Register(orders.CmdHold, real)
	if d.handlerFor(orders.CmdHold) != real {
		t.Error("registered command did not route to its handler")
	}
	if _, ok := d.handlerFor(orders.CmdMove).(stubHandler); !ok {
		t.Error("other commands should still route to the stub")
	}
}

// TestNewDispatchRegistersRealHandlers confirms the default dispatch ProcessTurn
// uses wires Hold and Move to their real handlers, while other commands still
// fall back to the stub.
func TestNewDispatchRegistersRealHandlers(t *testing.T) {
	d := NewDispatch()
	if _, ok := d.handlerFor(orders.CmdHold).(holdHandler); !ok {
		t.Errorf("hold routed to %T, want holdHandler", d.handlerFor(orders.CmdHold))
	}
	if _, ok := d.handlerFor(orders.CmdMove).(moveHandler); !ok {
		t.Errorf("move routed to %T, want moveHandler", d.handlerFor(orders.CmdMove))
	}
	if _, ok := d.handlerFor(orders.CmdExplore).(stubHandler); !ok {
		t.Errorf("explore routed to %T, want stubHandler (still stubbed)", d.handlerFor(orders.CmdExplore))
	}
}
