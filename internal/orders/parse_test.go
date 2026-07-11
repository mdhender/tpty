// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package orders

import (
	"strings"
	"testing"
)

// errStrings renders a slice of errors as their "line:col: msg" strings.
func errStrings(errs []Error) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Error()
	}
	return out
}

func TestParseValidFile(t *testing.T) {
	src := strings.Join([]string{
		`"smoke-test-1" 3 "k9m2qphtx7"`,
		``,
		`entity 101, "Conan the Copyright"`,
		`    drop  102`,
		`    move  1  2  3  2`,
		``,
		`entity 204, "King Loric"`,
		`    study 39 14`,
		`    form  9 16 2000 2`,
		`names:`,
		`    204 "The Slaves of Darkness"`,
		``,
	}, "\n")

	f, errs := Parse(src)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errStrings(errs))
	}
	if f.Opening == nil {
		t.Fatalf("opening record is nil")
	}
	if f.Opening.GameID != "smoke-test-1" || f.Opening.PlayerID != 3 || f.Opening.Password != "k9m2qphtx7" {
		t.Errorf("opening = %+v", *f.Opening)
	}
	if f.Opening.Line != 1 {
		t.Errorf("opening line = %d, want 1", f.Opening.Line)
	}
	if len(f.Entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(f.Entities))
	}

	e0 := f.Entities[0]
	if e0.EntityID != 101 || e0.Name != "Conan the Copyright" || e0.Line != 3 {
		t.Errorf("entity[0] = %+v", e0)
	}
	if len(e0.Orders) != 2 {
		t.Fatalf("entity[0] orders = %d, want 2", len(e0.Orders))
	}
	if e0.Orders[0].ID != CmdDrop || e0.Orders[0].Word != "drop" {
		t.Errorf("order[0] = %+v", e0.Orders[0])
	}
	if e0.Orders[1].ID != CmdMove || len(e0.Orders[1].Args) != 4 {
		t.Errorf("order[1] = %+v", e0.Orders[1])
	}

	e1 := f.Entities[1]
	if e1.EntityID != 204 || len(e1.Orders) != 2 {
		t.Errorf("entity[1] = %+v", e1)
	}
	if len(e1.Names) != 1 || e1.Names[0].EntityID != 204 || e1.Names[0].Name != "The Slaves of Darkness" {
		t.Errorf("entity[1] names = %+v", e1.Names)
	}
}

func TestParseOpeningRecord(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantNil    bool
		wantGameID string
		wantPlayer int
		wantPass   string
		wantErr    string // substring, empty means expect none
	}{
		{name: "unquoted", src: `g 3 pw`, wantGameID: "g", wantPlayer: 3, wantPass: "pw"},
		{name: "quoted", src: `"my game" 3 "p w"`, wantGameID: "my game", wantPlayer: 3, wantPass: "p w"},
		{name: "non-integer id", src: `g x pw`, wantNil: true, wantErr: `1:3: player id must be a positive integer, got "x"`},
		{name: "zero id", src: `g 0 pw`, wantNil: true, wantErr: `1:3: player id must be a positive integer, got "0"`},
		{name: "too few fields", src: `g 3`, wantNil: true, wantErr: `1:1: opening record has 2 field(s), expected 3`},
		{name: "too many fields", src: `g 3 pw extra`, wantNil: true, wantErr: `opening record has 4 field(s)`},
		{name: "unterminated quote", src: `g 3 "pw`, wantNil: true, wantErr: `1:5: unterminated quoted string`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, errs := Parse(tt.src)
			if tt.wantNil {
				if f.Opening != nil {
					t.Errorf("opening = %+v, want nil", *f.Opening)
				}
			} else {
				if f.Opening == nil {
					t.Fatalf("opening is nil, want record")
				}
				if f.Opening.GameID != tt.wantGameID || f.Opening.PlayerID != tt.wantPlayer || f.Opening.Password != tt.wantPass {
					t.Errorf("opening = %+v", *f.Opening)
				}
			}
			if tt.wantErr != "" {
				if !containsErr(errStrings(errs), tt.wantErr) {
					t.Errorf("errors %v do not contain %q", errStrings(errs), tt.wantErr)
				}
			}
		})
	}
}

func containsErr(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

// parseSingleOrder parses a one-entity file with a single order line and returns
// the order (if any) and the errors from that line.
func parseSingleOrder(t *testing.T, orderLine string) (Order, []Error) {
	t.Helper()
	src := "g 1 pw\nentity 1, x\n    " + orderLine + "\n"
	f, errs := Parse(src)
	if len(f.Entities) == 0 {
		t.Fatalf("no entity parsed for %q", orderLine)
	}
	if len(f.Entities[0].Orders) > 0 {
		return f.Entities[0].Orders[0], errs
	}
	return Order{}, errs
}

func TestParseArity(t *testing.T) {
	tests := []struct {
		line    string
		wantErr string // empty => expect no error
	}{
		{line: "hold"},
		{line: "hold 1", wantErr: "hold expects 0 arguments, got 1"},
		{line: "move 1"},
		{line: "move 1 2 3 4 5"},
		{line: "move", wantErr: "move expects at least 1 argument, got 0"},
		{line: "take 1"},
		{line: "take 1 2", wantErr: "take expects 1 argument, got 2"},
		{line: "take", wantErr: "take expects 1 argument, got 0"},
		{line: "attack"},
		{line: "attack 1"},
		{line: "attack 1 2", wantErr: "attack expects at most 1 argument, got 2"},
		{line: "use"},
		{line: "use 1 2 3"},
		{line: "use 1 2 3 4", wantErr: "use expects at most 3 arguments, got 4"},
		{line: "buy 1 2"},
		{line: "buy 1 2 3 4"},
		{line: "buy 1", wantErr: "buy expects at least 2 arguments, got 1"},
		{line: "pay 1 2 3"},
		{line: "pay 1 2", wantErr: "pay expects 3 arguments, got 2"},
		{line: "explore"},
		{line: "explore 1", wantErr: "explore expects 0 arguments, got 1"},
		{line: "form 1"},
		{line: "form 1 2 3 4"},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			_, errs := parseSingleOrder(t, tt.line)
			if tt.wantErr == "" {
				if len(errs) != 0 {
					t.Errorf("expected no error, got %v", errStrings(errs))
				}
			} else if !containsErr(errStrings(errs), tt.wantErr) {
				t.Errorf("errors %v do not contain %q", errStrings(errs), tt.wantErr)
			}
		})
	}
}

func TestParseAllCommandsKnown(t *testing.T) {
	// Every command word (including the tax alias) parses to an order at its
	// minimum arg count with no error.
	minLines := map[string]string{
		"hold": "hold", "move": "move 1", "attack": "attack", "use": "use",
		"take": "take 1", "drop": "drop", "join": "join 1", "study": "study 1",
		"work": "work", "buy": "buy 1 2", "sell": "sell 1 2", "follow": "follow",
		"explore": "explore", "persuade": "persuade 1", "swear": "swear",
		"pay": "pay 1 2 3", "declare": "declare 1", "recruit": "recruit 1 2",
		"form": "form 1", "pillage": "pillage (3,4)", "tax": "tax (3,4)",
		"execute": "execute 1", "terrorize": "terrorize", "wait": "wait",
		"armor": "armor", "tell": "tell 1", "garrison": "garrison",
	}
	for word, line := range minLines {
		t.Run(word, func(t *testing.T) {
			ord, errs := parseSingleOrder(t, line)
			if len(errs) != 0 {
				t.Errorf("%q: expected no error, got %v", line, errStrings(errs))
			}
			if ord.Word != word {
				t.Errorf("word = %q, want %q", ord.Word, word)
			}
		})
	}
	// Tax shares id 23 with pillage.
	ord, _ := parseSingleOrder(t, "tax (3,4)")
	if ord.ID != CmdPillage {
		t.Errorf("tax id = %d, want %d (pillage)", ord.ID, CmdPillage)
	}
}

func TestParseUnknownCommand(t *testing.T) {
	ord, errs := parseSingleOrder(t, "movr 1 2")
	if ord.Word != "" {
		t.Errorf("expected no order recorded, got %+v", ord)
	}
	if !containsErr(errStrings(errs), `3:5: unknown command "movr"`) {
		t.Errorf("errors %v", errStrings(errs))
	}
}

func TestParseProvinceParameters(t *testing.T) {
	tests := []struct {
		line    string
		wantErr string
	}{
		{line: "pillage (3,4)"},
		{line: "pillage (-1,0)"},
		{line: "pillage 5", wantErr: `pillage needs a province like (3,4), got "5"`},
		{line: "pillage (3,", wantErr: `malformed argument "(3,"`},
		{line: "pillage 3,4)", wantErr: `malformed argument "3,4)"`},
		{line: "tax 9", wantErr: `tax needs a province like (3,4), got "9"`},
		// terrorize's optional province position does not hard-error on a non-coord.
		{line: "terrorize 5"},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			_, errs := parseSingleOrder(t, tt.line)
			if tt.wantErr == "" {
				if len(errs) != 0 {
					t.Errorf("expected no error, got %v", errStrings(errs))
				}
			} else if !containsErr(errStrings(errs), tt.wantErr) {
				t.Errorf("errors %v do not contain %q", errStrings(errs), tt.wantErr)
			}
		})
	}
}

func TestParseMalformedArgumentReportedOnce(t *testing.T) {
	// A broken coordinate in a required province position reports the malformed
	// error, not also the "needs a province" error.
	_, errs := parseSingleOrder(t, "pillage (3,")
	if n := len(errStrings(errs)); n != 1 {
		t.Fatalf("expected exactly 1 error, got %d: %v", n, errStrings(errs))
	}
}

func TestParseCaseInsensitivity(t *testing.T) {
	src := strings.Join([]string{
		`g 1 pw`,
		`ENTITY 101, Conan`,
		`    Move 1 2`,
		`NAMES:`,
		`    101 Foo`,
	}, "\n")
	f, errs := Parse(src)
	if len(errs) != 0 {
		t.Fatalf("errors %v", errStrings(errs))
	}
	if len(f.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(f.Entities))
	}
	if f.Entities[0].Orders[0].Word != "move" {
		t.Errorf("word = %q, want move", f.Entities[0].Orders[0].Word)
	}
	if len(f.Entities[0].Names) != 1 {
		t.Errorf("names = %+v", f.Entities[0].Names)
	}
}

func TestParseHeaderCommaVariants(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		wantErr bool
	}{
		{name: "attached", header: `entity 101, "Conan"`},
		{name: "standalone", header: `entity 101 , "Conan"`},
		{name: "no space before name", header: `entity 101,"Conan"`},
		{name: "missing comma", header: `entity 101 "Conan"`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "g 1 pw\n" + tt.header + "\n    hold\n"
			f, errs := Parse(src)
			if len(f.Entities) != 1 {
				t.Fatalf("entities = %d, want 1", len(f.Entities))
			}
			if f.Entities[0].EntityID != 101 || f.Entities[0].Name != "Conan" {
				t.Errorf("entity = %+v", f.Entities[0])
			}
			// The block still holds its order regardless of the comma error.
			if len(f.Entities[0].Orders) != 1 {
				t.Errorf("orders = %d, want 1", len(f.Entities[0].Orders))
			}
			gotErr := len(errs) > 0
			if gotErr != tt.wantErr {
				t.Errorf("errors = %v, wantErr = %v", errStrings(errs), tt.wantErr)
			}
		})
	}
}

func TestParseRecoveryOrderLine(t *testing.T) {
	// A bad order line in the middle of a block still parses the surrounding
	// lines and the following entity block, reporting exactly one error.
	src := strings.Join([]string{
		`g 1 pw`,
		`entity 101, Conan`,
		`    hold`,
		`    movr 1 2`, // bad: unknown command
		`    explore`,
		`entity 202, Sendya`,
		`    work 1`,
	}, "\n")
	f, errs := Parse(src)
	if n := len(errs); n != 1 {
		t.Fatalf("expected 1 error, got %d: %v", n, errStrings(errs))
	}
	if len(f.Entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(f.Entities))
	}
	// hold and explore survive around the bad line.
	if len(f.Entities[0].Orders) != 2 {
		t.Errorf("entity[0] orders = %d, want 2", len(f.Entities[0].Orders))
	}
	if len(f.Entities[1].Orders) != 1 {
		t.Errorf("entity[1] orders = %d, want 1", len(f.Entities[1].Orders))
	}
}

func TestParseRecoveryBadHeader(t *testing.T) {
	// A bad entity header syncs to the next entity block. The order lines under
	// the bad header are not attached anywhere and raise no further errors.
	src := strings.Join([]string{
		`g 1 pw`,
		`entity xyz, Conan`, // bad id
		`    hold`,
		`    explore`,
		`entity 202, Sendya`,
		`    work 1`,
	}, "\n")
	f, errs := Parse(src)
	if n := len(errs); n != 1 {
		t.Fatalf("expected 1 error, got %d: %v", n, errStrings(errs))
	}
	if !containsErr(errStrings(errs), "entity id must be a positive integer") {
		t.Errorf("errors %v", errStrings(errs))
	}
	if len(f.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(f.Entities))
	}
	if f.Entities[0].EntityID != 202 || len(f.Entities[0].Orders) != 1 {
		t.Errorf("entity = %+v", f.Entities[0])
	}
}

func TestParseOrderOutsideEntityBlock(t *testing.T) {
	src := "g 1 pw\n    hold\nentity 1, x\n    move 1\n"
	f, errs := Parse(src)
	if !containsErr(errStrings(errs), "order line outside an entity block") {
		t.Errorf("errors %v", errStrings(errs))
	}
	// The later entity still parses.
	if len(f.Entities) != 1 || len(f.Entities[0].Orders) != 1 {
		t.Errorf("entities = %+v", f.Entities)
	}
}

func TestParseColumns(t *testing.T) {
	// Indented order line: the command word starts at column 5.
	ord, _ := parseSingleOrder(t, "hold")
	if ord.Col != 5 {
		t.Errorf("indented command col = %d, want 5", ord.Col)
	}

	// Error points at the start of the offending mid-line field.
	src := "g 1 pw\nentity 1, x\n    pillage 5\n"
	_, errs := Parse(src)
	want := `3:13: pillage needs a province like (3,4), got "5"`
	if !containsErr(errStrings(errs), want) {
		t.Errorf("errors %v do not contain %q", errStrings(errs), want)
	}
}

func TestParseEmptyAndBlankFiles(t *testing.T) {
	for _, src := range []string{"", "\n\n", "   \n\t\n"} {
		f, errs := Parse(src)
		if f.Opening != nil {
			t.Errorf("src %q: opening = %+v, want nil", src, *f.Opening)
		}
		if len(errs) != 0 {
			t.Errorf("src %q: errors %v", src, errStrings(errs))
		}
	}
}
