// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Package orders is the pure parser for T'Pty orders files. It turns source
// text into an abstract syntax tree ([File]) plus a list of friendly,
// positioned errors ([]Error). It is deliberately free of game state: it does
// not know which players, factions, or entities exist, and it imports nothing
// from package tpty. Authentication and ownership are layered on top by package
// tpty.
//
// The grammar it implements is documented in
// content/docs/reference/orders/_index.md and its grammar.md; the parser
// contract (error format, multi-error recovery, and what is checked versus
// deferred) is documented in content/docs/reference/orders/parsing.md.
package orders

import "fmt"

// Error is a parse or validation problem at a source position. Line and Col are
// both 1-based; Col is a rune (character) position, not a byte offset. Error
// renders as "line:col: message".
type Error struct {
	Line int
	Col  int
	Msg  string
}

// Error implements the error interface, rendering "line:col: message".
func (e Error) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg)
}

// OpeningRecord is the parsed opening record that authenticates an orders file:
// the game id, the player id, and the player's password. Line is the 1-based
// line the record was read from. Text fields have had their surrounding quotes
// removed.
type OpeningRecord struct {
	GameID   string
	PlayerID int
	Password string
	Line     int
}

// CommandID is the canonical numeric id of an order command (0..29). Tax shares
// id 23 with Pillage. Ids 7, 13, 17, and 22 are unused.
type CommandID int

// The canonical command ids. These are a frozen numbering; do not renumber.
const (
	CmdHold      CommandID = 0
	CmdMove      CommandID = 1
	CmdAttack    CommandID = 2
	CmdUse       CommandID = 3
	CmdTake      CommandID = 4
	CmdDrop      CommandID = 5
	CmdJoin      CommandID = 6
	CmdStudy     CommandID = 8
	CmdWork      CommandID = 9
	CmdBuy       CommandID = 10
	CmdSell      CommandID = 11
	CmdFollow    CommandID = 12
	CmdExplore   CommandID = 14
	CmdPersuade  CommandID = 15
	CmdSwear     CommandID = 16
	CmdPay       CommandID = 18
	CmdDeclare   CommandID = 19
	CmdRecruit   CommandID = 20
	CmdForm      CommandID = 21
	CmdPillage   CommandID = 23 // Tax is an alias of Pillage and shares this id.
	CmdExecute   CommandID = 24
	CmdTerrorize CommandID = 25
	CmdWait      CommandID = 26
	CmdArmor     CommandID = 27
	CmdTell      CommandID = 28
	CmdGarrison  CommandID = 29
)

// Order is one parsed order line: its canonical id, the command word as written
// (lower-cased), its raw argument fields (quotes removed, in order), and its
// source position. Col is the 1-based rune column of the command word.
//
// The arguments are stored raw: the parser validates only that each is a
// syntactically well-formed field (see parsing.md). Interpreting a number as a
// direction, skill, or thing is deferred to execution.
type Order struct {
	ID   CommandID
	Word string
	Args []string
	Line int
	Col  int
}

// NameAssignment is one entry in a names section: the id of the forming entity
// and the name to give the unit it forms.
type NameAssignment struct {
	EntityID int
	Name     string
	Line     int
}

// EntityBlock is a parsed entity block: the entity's id and name, the header's
// line, the order lines in the block, and any name assignments from a trailing
// names section.
type EntityBlock struct {
	EntityID int
	Name     string
	Line     int
	Orders   []Order
	Names    []NameAssignment
}

// File is the parsed form of a whole orders file: the opening record (nil if it
// could not be parsed) and one EntityBlock per entity block the parser
// recovered.
type File struct {
	Opening  *OpeningRecord
	Entities []EntityBlock
}

// command describes one entry in the command table: the canonical id, the
// minimum and maximum argument counts (maxArgs == -1 means unbounded), and the
// 0-based argument positions that must be a province coordinate.
type command struct {
	id      CommandID
	minArgs int
	maxArgs int
	reqProv []int
}

// commandTable maps each command word to its entry. Tax is an alias of Pillage
// and shares id 23. The table is the parsing specification for arity and
// required province parameters.
var commandTable = map[string]command{
	"hold":      {id: CmdHold, minArgs: 0, maxArgs: 0},
	"move":      {id: CmdMove, minArgs: 1, maxArgs: -1},
	"attack":    {id: CmdAttack, minArgs: 0, maxArgs: 1},
	"use":       {id: CmdUse, minArgs: 0, maxArgs: 3},
	"take":      {id: CmdTake, minArgs: 1, maxArgs: 1},
	"drop":      {id: CmdDrop, minArgs: 0, maxArgs: 1},
	"join":      {id: CmdJoin, minArgs: 1, maxArgs: 1},
	"study":     {id: CmdStudy, minArgs: 1, maxArgs: 2},
	"work":      {id: CmdWork, minArgs: 0, maxArgs: 2},
	"buy":       {id: CmdBuy, minArgs: 2, maxArgs: 4},
	"sell":      {id: CmdSell, minArgs: 2, maxArgs: 4},
	"follow":    {id: CmdFollow, minArgs: 0, maxArgs: 1},
	"explore":   {id: CmdExplore, minArgs: 0, maxArgs: 0},
	"persuade":  {id: CmdPersuade, minArgs: 1, maxArgs: 3},
	"swear":     {id: CmdSwear, minArgs: 0, maxArgs: 1},
	"pay":       {id: CmdPay, minArgs: 3, maxArgs: 3},
	"declare":   {id: CmdDeclare, minArgs: 1, maxArgs: 2},
	"recruit":   {id: CmdRecruit, minArgs: 2, maxArgs: 2},
	"form":      {id: CmdForm, minArgs: 1, maxArgs: 4},
	"pillage":   {id: CmdPillage, minArgs: 1, maxArgs: 2, reqProv: []int{0}},
	"tax":       {id: CmdPillage, minArgs: 1, maxArgs: 2, reqProv: []int{0}},
	"execute":   {id: CmdExecute, minArgs: 1, maxArgs: 1},
	"terrorize": {id: CmdTerrorize, minArgs: 0, maxArgs: 3},
	"wait":      {id: CmdWait, minArgs: 0, maxArgs: 1},
	"armor":     {id: CmdArmor, minArgs: 0, maxArgs: 1},
	"tell":      {id: CmdTell, minArgs: 1, maxArgs: 3},
	"garrison":  {id: CmdGarrison, minArgs: 0, maxArgs: 1},
}
