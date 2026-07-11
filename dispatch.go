// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"

	"github.com/mdhender/tpty/internal/orders"
)

// GameState bundles the read/write model an order handler operates over: the
// game (its seeds, turn, and files) and the entity and faction stores. It is
// passed to handlers so a real command (arriving in burndown item 15) can read
// and mutate world state without the engine growing a wide parameter list.
//
// See content/docs/reference/turn-processing.md.
type GameState struct {
	Game     *Game
	Entities *EntityStore
	Factions *FactionStore
}

// Handler executes one command. The engine calls Cost when the order becomes
// active — to learn how many ticks it should occupy — and Apply when the order
// completes, to apply its effects. Turn and tick locate the call in the
// timeline so a handler can seed randomness by turn+tick (see
// content/docs/reference/determinism.md); the stub handler draws none.
//
// See content/docs/reference/turn-processing.md.
type Handler interface {
	// Cost reports the order's time cost in ticks at the moment it becomes the
	// active order. A return of 0 is a zero-time order that completes in the
	// tick it becomes active, consuming no working tick.
	Cost(s GameState, e Entity, o orders.Order, turn, tick int) int
	// Apply applies the order's effects when it completes (ticks-left reaches 0)
	// and returns an outcome describing what happened.
	Apply(s GameState, e Entity, o orders.Order, turn, tick int) OrderOutcome
}

// commandMeta is one command's fixed scheduling and timing properties: its
// scheduler priority and its base time cost in ticks. varies records that the
// reference lists the cost as "varies" (or "0+"), meaning the real cost depends
// on the order's parameters or outcome and is the handler's responsibility; the
// stub reports the provisional cost stored here.
//
// See content/docs/reference/turn-processing.md and
// content/docs/reference/orders/_index.md ("Command summary").
type commandMeta struct {
	priority int  // 1..5 per the turn-processing category table; 0 is the reserved defect sentinel.
	cost     int  // base/provisional time cost in ticks.
	varies   bool // the reference lists this command's cost as "varies" or "0+".
}

// stubVariesCost is the provisional time cost, in ticks, the stub handler
// reports for a command whose real cost "varies". It is one week — a single
// standard action — matching the fixed 7-day cost of Hold, Explore, and Work in
// content/docs/reference/orders/_index.md and the "7-day move" worked example in
// content/docs/reference/turn-processing.md. This is a stub placeholder; item 15
// replaces it with each command's real per-parameter cost. Tell is the sole
// exception: its cost is listed "0+", so its provisional base is 0 (see the
// table below).
const stubVariesCost = 7

// commandMetaTable maps every canonical CommandID to its scheduling priority and
// base time cost. Priorities follow the category table in
// content/docs/reference/turn-processing.md: permission commands = 1 (none are
// defined yet); zero-time commands and wait = 2; move and fly = 3 (fly is not
// yet defined); all other commands = 4; sail = 5 (not yet defined). Base costs
// are the day values in content/docs/reference/orders/_index.md, taken as ticks;
// "varies"/"0+" costs use the provisional stubVariesCost (0 for Tell).
//
// The table is looked up by key only; it is never ranged over in a way that
// affects resolution order or random draws (CLAUDE.md). Ids 7, 13, 17, and 22
// are unused; Tax aliases Pillage at id 23 and needs no separate entry.
var commandMetaTable = map[orders.CommandID]commandMeta{
	orders.CmdHold:      {priority: 4, cost: 7},
	orders.CmdMove:      {priority: 3, cost: stubVariesCost, varies: true},
	orders.CmdAttack:    {priority: 4, cost: stubVariesCost, varies: true},
	orders.CmdUse:       {priority: 4, cost: stubVariesCost, varies: true},
	orders.CmdTake:      {priority: 4, cost: 7},
	orders.CmdDrop:      {priority: 2, cost: 0},
	orders.CmdJoin:      {priority: 2, cost: 0},
	orders.CmdStudy:     {priority: 4, cost: stubVariesCost, varies: true},
	orders.CmdWork:      {priority: 4, cost: 7},
	orders.CmdBuy:       {priority: 4, cost: 7},
	orders.CmdSell:      {priority: 4, cost: 7},
	orders.CmdFollow:    {priority: 4, cost: 28},
	orders.CmdExplore:   {priority: 4, cost: 7},
	orders.CmdPersuade:  {priority: 4, cost: 7},
	orders.CmdSwear:     {priority: 2, cost: 0},
	orders.CmdPay:       {priority: 2, cost: 0},
	orders.CmdDeclare:   {priority: 2, cost: 0},
	orders.CmdRecruit:   {priority: 4, cost: 14},
	orders.CmdForm:      {priority: 2, cost: 0},
	orders.CmdPillage:   {priority: 4, cost: 7},
	orders.CmdExecute:   {priority: 4, cost: 28},
	orders.CmdTerrorize: {priority: 4, cost: 7},
	orders.CmdWait:      {priority: 2, cost: stubVariesCost, varies: true},
	orders.CmdArmor:     {priority: 4, cost: stubVariesCost, varies: true},
	orders.CmdTell:      {priority: 2, cost: 0, varies: true}, // "0+": provisional base 0.
	orders.CmdGarrison:  {priority: 2, cost: 0},
}

// commandMetaFor returns the metadata for a command id. The ok result is false
// for an id with no entry (an unused id, or a defect), in which case the
// returned metadata is the zero value — priority 0, the reserved defect
// sentinel the scheduler flags.
func commandMetaFor(id orders.CommandID) (commandMeta, bool) {
	m, ok := commandMetaTable[id]
	return m, ok
}

// Dispatch routes each command id to the Handler that executes it. Until real
// handlers land (burndown item 15) every id resolves to the stub no-op handler,
// so an unimplemented order is recorded, not an error.
//
// See content/docs/reference/turn-processing.md.
type Dispatch struct {
	handlers map[orders.CommandID]Handler
	stub     Handler
}

// NewDispatch returns the default dispatch table ProcessTurn uses: the real
// handlers implemented so far (Hold and Move) are registered, and every other
// command id falls back to the stub no-op handler. Register replaces or adds
// individual entries as further real handlers are implemented.
func NewDispatch() *Dispatch {
	d := &Dispatch{
		handlers: make(map[orders.CommandID]Handler),
		stub:     stubHandler{},
	}
	d.Register(orders.CmdHold, holdHandler{})
	d.Register(orders.CmdMove, moveHandler{})
	return d
}

// newStubDispatch returns a dispatch table in which every command id routes to
// the stub no-op handler, with no real handlers registered. The engine's
// mechanical and scheduler tests use it so their assertions about the timeline
// do not depend on which commands have real handlers.
func newStubDispatch() *Dispatch {
	return &Dispatch{
		handlers: make(map[orders.CommandID]Handler),
		stub:     stubHandler{},
	}
}

// Register binds a command id to a handler, replacing any existing binding. It
// returns the dispatch table so registrations can be chained.
func (d *Dispatch) Register(id orders.CommandID, h Handler) *Dispatch {
	d.handlers[id] = h
	return d
}

// handlerFor returns the handler bound to id, or the stub no-op handler when no
// handler is registered. The lookup is by key only and does not depend on map
// iteration order.
func (d *Dispatch) handlerFor(id orders.CommandID) Handler {
	if h, ok := d.handlers[id]; ok {
		return h
	}
	return d.stub
}

// stubHandler is the default no-op handler. It reports a command's base time
// cost (the provisional stubVariesCost for "varies"/"0+" commands) and applies
// no effects, recording that the order was processed as a stub rather than
// failing. Every command id routes here until item 15 supplies real handlers. It
// draws no randomness, so this item adds no new prng domain tag.
//
// See content/docs/reference/turn-processing.md.
type stubHandler struct{}

// Cost reports the command's base/provisional cost from the command metadata
// table. An unknown id reports 0 so the order does not stall the timeline; the
// scheduler separately flags the priority-0 defect.
func (stubHandler) Cost(_ GameState, _ Entity, o orders.Order, _, _ int) int {
	m, ok := commandMetaFor(o.ID)
	if !ok {
		return 0
	}
	return m.cost
}

// Apply records a no-op: the order was processed as a stub and changed nothing.
func (stubHandler) Apply(_ GameState, e Entity, o orders.Order, _, _ int) OrderOutcome {
	return OrderOutcome{
		EntityID: e.ID,
		Order:    o,
		Stub:     true,
		Message:  fmt.Sprintf("%s: processed as stub (no effect)", o.Word),
	}
}
