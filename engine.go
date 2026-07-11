// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package tpty

import (
	"fmt"
	"sort"

	"github.com/mdhender/tpty/internal/orders"
)

// The tick timeline of a processed turn: a turn runs as 32 ticks, 0..31. Tick 0
// is setup, ticks 1..30 are the working ticks that advance active orders, and
// tick 31 is cleanup (it lets an order that completes on tick 30 land within the
// turn). See content/docs/reference/turn-processing.md ("Ticks").
const (
	setupTick     = 0
	firstWorkTick = 1
	lastWorkTick  = 30
	cleanupTick   = 31
)

// EntityQueue is one entity's pending order queue: its FIFO list of orders with
// the front order first. TicksLeft and Active describe the front (active) order
// so an unfinished order resumes next turn with its remaining ticks: Active
// reports whether the front order has already been activated (its cost fixed),
// and TicksLeft is its remaining ticks when it has. The engine consumes these as
// carried-in queues and returns them as carryover.
//
// See content/docs/reference/turn-processing.md ("Order queue", "Working an
// order").
type EntityQueue struct {
	EntityID  int
	Orders    []orders.Order
	Active    bool
	TicksLeft int
}

// OrderOutcome records what happened to one order during processing: the acting
// entity, the order itself, whether it was handled by the stub no-op (as opposed
// to a real handler), and a human-readable message for the turn report.
//
// See content/docs/reference/turn-processing.md.
type OrderOutcome struct {
	EntityID int
	Order    orders.Order
	Stub     bool
	Message  string
}

// TurnInput is the whole deterministic input to ProcessTurn: the game state, the
// turn being processed, each player's submitted orders (the verbatim
// StoredOrders.Raw, re-parsed here), any queues carried in from the previous
// turn, and the command dispatch table. Given identical inputs ProcessTurn
// returns an identical result.
//
// See content/docs/reference/turn-processing.md.
type TurnInput struct {
	State     GameState
	Turn      int
	Submitted []StoredOrders
	Carryover []EntityQueue
	Dispatch  *Dispatch
}

// TurnResult is the structured, deterministic result of processing one turn: the
// turn number, the per-order outcomes in the order they completed, the carryover
// queues (unfinished orders, to be carried into the next turn), and a processing
// log detailed enough for the turn writer (burndown item 16) to render.
//
// See content/docs/reference/turn-processing.md.
type TurnResult struct {
	Turn      int
	Outcomes  []OrderOutcome
	Carryover []EntityQueue
	Log       []string
}

// entityQueue is the engine's private, mutable view of one entity's order queue
// during a turn. orders[0] is the active order; active reports whether it has
// been activated (ticksLeft set from its handler's Cost).
type entityQueue struct {
	entityID  int
	orders    []orders.Order
	active    bool
	ticksLeft int
}

// scheduleItem is one entity's slot in a working tick's total resolution order.
// The keys are read in order — priority, then location q, then r, then seniority
// — to sort every active entity into one deterministic sequence.
type scheduleItem struct {
	q         *entityQueue
	entity    Entity
	priority  int
	locQ      int
	locR      int
	seniority int // index in the entity store; lower is more senior.
}

// ProcessTurn runs one turn of the execution engine over the given state and the
// players' submitted orders, and returns a deterministic result. It performs no
// I/O: it re-parses each StoredOrders.Raw with internal/orders, builds each
// entity's FIFO order queue (carryover first, then newly submitted orders
// appended to the back), advances the 32-tick timeline, resolving active orders
// each working tick in one total order (priority, then location, then
// seniority), and reports the outcomes and any carryover. Identical inputs
// always produce an identical result.
//
// See content/docs/reference/turn-processing.md.
func ProcessTurn(in TurnInput) TurnResult {
	dispatch := in.Dispatch
	if dispatch == nil {
		dispatch = NewDispatch()
	}

	res := TurnResult{Turn: in.Turn}
	logf := func(format string, args ...any) {
		res.Log = append(res.Log, fmt.Sprintf(format, args...))
	}

	// Tick 0 — setup: build every entity's order queue.
	logf("tick %d: setup", setupTick)
	queues := buildQueues(in, logf)

	// Ticks 1..30 — work: advance active orders in scheduler order each tick.
	for tick := firstWorkTick; tick <= lastWorkTick; tick++ {
		schedule := buildSchedule(queues, in.State.Entities, logf)
		for _, item := range schedule {
			workEntity(item.q, in, dispatch, tick, &res)
		}
	}

	// Tick 31 — cleanup: any order still carrying ticks-left resumes next turn.
	logf("tick %d: cleanup", cleanupTick)
	res.Carryover = collectCarryover(queues, in.State.Entities)
	for _, cq := range res.Carryover {
		logf("carryover: entity %d has %d order(s) queued (active ticks-left %d)",
			cq.EntityID, len(cq.Orders), cq.TicksLeft)
	}

	return res
}

// buildQueues constructs each entity's order queue for the turn: carried-in
// orders first (so they finish before newly submitted ones), then the turn's
// newly submitted orders appended to the back. Submitted orders are gathered by
// re-parsing each StoredOrders.Raw. Queues are keyed by entity id for
// accumulation but the keyed map is never ranged over in a way that affects
// order: reads walk the entity store's stable slice order instead.
func buildQueues(in TurnInput, logf func(string, ...any)) map[int]*entityQueue {
	queues := make(map[int]*entityQueue)

	queueFor := func(entityID int) *entityQueue {
		q, ok := queues[entityID]
		if !ok {
			q = &entityQueue{entityID: entityID}
			queues[entityID] = q
		}
		return q
	}

	// Carryover first, preserving each front order's activation and ticks-left.
	for _, cq := range in.Carryover {
		if len(cq.Orders) == 0 {
			continue
		}
		q := queueFor(cq.EntityID)
		q.orders = append(q.orders, cq.Orders...)
		if !q.active {
			q.active = cq.Active
			q.ticksLeft = cq.TicksLeft
		}
	}

	// Newly submitted orders, appended to the back in a deterministic order: by
	// the submitted slice, then by parsed block, then by order line.
	for _, so := range in.Submitted {
		file, errs := orders.Parse(so.Raw)
		for _, e := range errs {
			logf("player %d turn %d: parse error %s", so.PlayerID, so.Turn, e.Error())
		}
		for _, block := range file.Entities {
			q := queueFor(block.EntityID)
			q.orders = append(q.orders, block.Orders...)
		}
	}

	return queues
}

// buildSchedule gathers every entity with a non-empty queue into the working
// tick's total resolution order. It sorts by the four documented keys in turn:
// the active order's priority (ascending), the entity's location (q ascending,
// then r ascending), and seniority. It walks the entity store's stable slice so
// the input order never depends on Go map iteration (CLAUDE.md).
//
// Seniority is the entity's position in its location's FIFO arrival list. Until
// provinces model that list explicitly, it is derived from the entity's stable
// position in the entity store within the same location — the earlier-stored
// entity is the more senior (see content/docs/reference/turn-processing.md,
// "Seniority", and the store-order interpretation noted there).
func buildSchedule(queues map[int]*entityQueue, entities *EntityStore, logf func(string, ...any)) []scheduleItem {
	var items []scheduleItem
	for idx, e := range entities.Entities {
		q, ok := queues[e.ID]
		if !ok || len(q.orders) == 0 {
			continue
		}
		active := q.orders[0]
		meta, ok := commandMetaFor(active.ID)
		if !ok || meta.priority == 0 {
			// Priority 0 is the reserved defect sentinel: an order reached the
			// scheduler with no priority (an unused or unknown command id). Flag
			// it; a real command always carries a priority, so this never fires
			// for parsed input.
			logf("defect: entity %d order %q (id %d) has no scheduler priority",
				e.ID, active.Word, active.ID)
		}
		loc, err := canonicalProvince(e.Location)
		if err != nil {
			logf("defect: entity %d has non-canonical location %q: %v", e.ID, e.Location, err)
		}
		items = append(items, scheduleItem{
			q:         q,
			entity:    e,
			priority:  meta.priority,
			locQ:      loc.Q,
			locR:      loc.R,
			seniority: idx,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.priority != b.priority {
			return a.priority < b.priority
		}
		if a.locQ != b.locQ {
			return a.locQ < b.locQ
		}
		if a.locR != b.locR {
			return a.locR < b.locR
		}
		return a.seniority < b.seniority
	})
	return items
}

// workEntity advances one entity's active order by one working tick. It first
// activates the front order if needed (fixing its ticks-left from the handler's
// Cost) and completes any zero-time orders at the front, which consume no
// working tick; it then spends the tick on the first order that carries
// ticks-left, completing it if the tick brings ticks-left to zero. Completions
// record an outcome via the order's handler.
func workEntity(q *entityQueue, in TurnInput, dispatch *Dispatch, tick int, res *TurnResult) {
	entity := lookupEntity(in.State.Entities, q.entityID)

	activate := func() {
		if q.active || len(q.orders) == 0 {
			return
		}
		h := dispatch.handlerFor(q.orders[0].ID)
		q.ticksLeft = h.Cost(in.State, entity, q.orders[0], in.Turn, tick)
		q.active = true
	}

	complete := func() {
		o := q.orders[0]
		h := dispatch.handlerFor(o.ID)
		outcome := h.Apply(in.State, entity, o, in.Turn, tick)
		res.Outcomes = append(res.Outcomes, outcome)
		res.Log = append(res.Log, fmt.Sprintf("tick %d: entity %d completed %q — %s",
			tick, q.entityID, o.Word, outcome.Message))
		q.orders = q.orders[1:]
		q.active = false
		q.ticksLeft = 0
	}

	// Activate the front order, then complete any zero-time orders — each
	// completes in the tick it becomes active and consumes no working tick, so a
	// run of zero-time orders all clear in this tick.
	activate()
	for q.active && q.ticksLeft == 0 && len(q.orders) > 0 {
		complete()
		activate()
	}
	if len(q.orders) == 0 {
		return
	}

	// Spend the working tick on the now-active order carrying ticks-left.
	q.ticksLeft--
	if q.ticksLeft == 0 {
		complete()
	}
}

// collectCarryover returns the queues that still hold orders at the end of the
// turn, walking the entity store's stable slice so the result order is
// deterministic. An entity whose queue emptied contributes nothing.
func collectCarryover(queues map[int]*entityQueue, entities *EntityStore) []EntityQueue {
	var out []EntityQueue
	for _, e := range entities.Entities {
		q, ok := queues[e.ID]
		if !ok || len(q.orders) == 0 {
			continue
		}
		out = append(out, EntityQueue{
			EntityID:  q.entityID,
			Orders:    append([]orders.Order(nil), q.orders...),
			Active:    q.active,
			TicksLeft: q.ticksLeft,
		})
	}
	return out
}

// lookupEntity returns the entity with the given id, or a zero Entity carrying
// just the id if the store has no such entity (which upstream ownership checks
// should already have prevented).
func lookupEntity(entities *EntityStore, id int) Entity {
	if e, ok := entities.ByID(id); ok {
		return e
	}
	return Entity{ID: id}
}
