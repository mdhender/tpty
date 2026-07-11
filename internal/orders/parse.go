// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package orders

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Parse parses an orders file. It always returns a File with as much structure
// as it could recover, plus every error it found (nil/empty when the file is
// clean).
//
// Parsing never stops at the first error. A malformed order line recovers at the
// start of the next line; a malformed entity header recovers at the start of the
// next entity (the next line whose first word is "entity" or "names:", or end of
// file). See content/docs/reference/orders/parsing.md for the contract.
func Parse(src string) (*File, []Error) {
	f := &File{}
	var errs []Error

	lines := splitLines(src)

	// State machine over the lines after the opening record.
	curIdx := -1      // index into f.Entities of the current block, or -1
	skipping := false // recovering after a bad header or bad opening record
	inNames := false  // reading a names section for the current block
	openingDone := false

	for i, raw := range lines {
		lineNo := i + 1

		if isBlank(raw) {
			// A blank line separates records and ends a names section.
			inNames = false
			continue
		}

		if !openingDone {
			// The first non-blank line is the opening record.
			openingDone = true
			rec, recErrs := parseOpening(raw, lineNo)
			errs = append(errs, recErrs...)
			f.Opening = rec
			// If the opening record is unusable, recover at the next entity.
			skipping = rec == nil
			continue
		}

		fields, lexErrs := splitFields(raw, lineNo)
		if len(fields) == 0 {
			// Only lexer noise (e.g. an unterminated quote with no content).
			errs = append(errs, lexErrs...)
			continue
		}
		first := strings.ToLower(fields[0].text)

		switch {
		case first == "entity":
			errs = append(errs, lexErrs...)
			inNames = false
			block, hdrErrs, ok := parseHeader(fields, lineNo)
			errs = append(errs, hdrErrs...)
			if ok {
				f.Entities = append(f.Entities, block)
				curIdx = len(f.Entities) - 1
				skipping = false
			} else {
				// Bad header: drop the current block target and skip its lines.
				curIdx = -1
				skipping = true
			}

		case first == "names:":
			errs = append(errs, lexErrs...)
			if curIdx >= 0 {
				inNames = true
				skipping = false
			} else {
				// A names section with no entity to attach to.
				errs = append(errs, Error{Line: lineNo, Col: fields[0].col,
					Msg: "names section outside an entity block"})
				inNames = false
				skipping = true
			}

		case inNames:
			errs = append(errs, lexErrs...)
			na, naErrs, ok := parseNameAssignment(fields, lineNo)
			errs = append(errs, naErrs...)
			if ok && curIdx >= 0 {
				f.Entities[curIdx].Names = append(f.Entities[curIdx].Names, na)
			}

		case skipping:
			// Recovering after a bad header/opening record: silently skip.

		case curIdx < 0:
			errs = append(errs, lexErrs...)
			errs = append(errs, Error{Line: lineNo, Col: fields[0].col,
				Msg: "order line outside an entity block"})

		default:
			errs = append(errs, lexErrs...)
			ord, ordErrs, ok := parseOrder(fields, lineNo)
			errs = append(errs, ordErrs...)
			if ok {
				f.Entities[curIdx].Orders = append(f.Entities[curIdx].Orders, ord)
			}
		}
	}

	if len(errs) == 0 {
		errs = nil
	}
	return f, errs
}

// field is one tokenized field of a line: its text (surrounding quotes removed),
// its 1-based rune start column, and whether it was written quoted.
type field struct {
	text   string
	col    int
	quoted bool
}

// splitLines splits src into lines, tolerating LF and CRLF endings. A trailing
// newline does not produce a final empty line.
func splitLines(src string) []string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	if src == "" {
		return nil
	}
	lines := strings.Split(src, "\n")
	// Drop a single trailing empty element produced by a terminating newline.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// isBlank reports whether a line is empty or all whitespace.
func isBlank(line string) bool {
	return strings.TrimSpace(line) == ""
}

// splitFields splits a single line into whitespace-separated fields, recording
// each field's 1-based rune start column. Whitespace is a space or a tab. A
// field that begins with a double quote runs to the next double quote; an
// unterminated quote is reported as an error at the quote's column (and the
// partial field is still returned).
func splitFields(line string, lineNo int) ([]field, []Error) {
	var fields []field
	var errs []Error
	runes := []rune(line)
	n := len(runes)
	i := 0
	for i < n {
		for i < n && isSpace(runes[i]) {
			i++
		}
		if i >= n {
			break
		}
		startCol := i + 1
		if runes[i] == '"' {
			j := i + 1
			for j < n && runes[j] != '"' {
				j++
			}
			if j >= n {
				errs = append(errs, Error{Line: lineNo, Col: startCol,
					Msg: "unterminated quoted string"})
				fields = append(fields, field{text: string(runes[i+1 : j]), col: startCol, quoted: true})
				i = j
			} else {
				fields = append(fields, field{text: string(runes[i+1 : j]), col: startCol, quoted: true})
				i = j + 1
			}
			continue
		}
		j := i
		for j < n && !isSpace(runes[j]) {
			j++
		}
		fields = append(fields, field{text: string(runes[i:j]), col: startCol})
		i = j
	}
	return fields, errs
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' }

// stripQuotes removes a single pair of surrounding double quotes from s, if
// present. It is used for name text recovered from a field the whitespace
// tokenizer did not itself unquote (a name written with no space after the
// comma, e.g. `entity 101,"Conan"`).
func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseOpening parses the opening record from a line. It returns a non-nil
// record only when the game id, a positive integer player id, and a password
// were all read; otherwise it returns nil and one or more errors.
func parseOpening(line string, lineNo int) (*OpeningRecord, []Error) {
	fields, errs := splitFields(line, lineNo)
	if len(errs) > 0 {
		// An unterminated quote makes the record unusable; still report field
		// count if it is also wrong, but never build a record.
		return nil, errs
	}
	if len(fields) != 3 {
		col := 1
		if len(fields) > 0 {
			col = fields[0].col
		}
		errs = append(errs, Error{Line: lineNo, Col: col,
			Msg: fmt.Sprintf("opening record has %d field(s), expected 3 (game id, player id, password)", len(fields))})
		return nil, errs
	}
	id, err := strconv.Atoi(fields[1].text)
	if err != nil || id < 1 {
		errs = append(errs, Error{Line: lineNo, Col: fields[1].col,
			Msg: fmt.Sprintf("player id must be a positive integer, got %q", fields[1].text)})
		return nil, errs
	}
	return &OpeningRecord{
		GameID:   fields[0].text,
		PlayerID: id,
		Password: fields[2].text,
		Line:     lineNo,
	}, errs
}

// parseHeader parses an entity header line whose first field is the "entity"
// keyword. It returns the block, any errors, and whether the id was usable. The
// comma after the id is accepted attached to the id, standalone, or leading the
// name; a missing comma is a reported-but-recoverable error. A non-positive or
// non-integer id makes the block unusable (ok == false).
func parseHeader(fields []field, lineNo int) (EntityBlock, []Error, bool) {
	var errs []Error
	idx := 1
	if idx >= len(fields) {
		errs = append(errs, Error{Line: lineNo, Col: fields[0].col,
			Msg: "entity header is missing an id"})
		return EntityBlock{}, errs, false
	}

	idField := fields[idx]
	idText := idField.text
	commaSeen := false
	haveName := false
	var nameText string

	// A comma attached to the id field: "101," or "101,Conan".
	if k := strings.IndexByte(idText, ','); k >= 0 {
		commaSeen = true
		after := idText[k+1:]
		idText = idText[:k]
		if after != "" {
			nameText = stripQuotes(after)
			haveName = true
		}
	}

	id, err := strconv.Atoi(idText)
	if err != nil || id < 1 {
		errs = append(errs, Error{Line: lineNo, Col: idField.col,
			Msg: fmt.Sprintf("entity id must be a positive integer, got %q", idText)})
		return EntityBlock{}, errs, false
	}
	idx++

	if !commaSeen {
		switch {
		case idx < len(fields) && fields[idx].text == ",":
			commaSeen = true
			idx++
		case idx < len(fields) && strings.HasPrefix(fields[idx].text, ","):
			commaSeen = true
			after := fields[idx].text[1:]
			if after != "" {
				nameText = stripQuotes(after)
				haveName = true
			}
			idx++
		default:
			errs = append(errs, Error{Line: lineNo, Col: idField.col + utf8.RuneCountInString(idText),
				Msg: "missing comma after entity id"})
		}
	}

	if !haveName && idx < len(fields) {
		nameText = stripQuotes(strings.TrimPrefix(fields[idx].text, ","))
		haveName = true
		idx++
	}

	if !haveName {
		errs = append(errs, Error{Line: lineNo, Col: idField.col,
			Msg: "entity header is missing a name"})
	}

	return EntityBlock{EntityID: id, Name: nameText, Line: lineNo}, errs, true
}

// parseNameAssignment parses one line of a names section: "<entityId> <name>".
func parseNameAssignment(fields []field, lineNo int) (NameAssignment, []Error, bool) {
	var errs []Error
	id, err := strconv.Atoi(fields[0].text)
	if err != nil || id < 1 {
		errs = append(errs, Error{Line: lineNo, Col: fields[0].col,
			Msg: fmt.Sprintf("names entry must start with an entity id, got %q", fields[0].text)})
		return NameAssignment{}, errs, false
	}
	if len(fields) < 2 {
		errs = append(errs, Error{Line: lineNo, Col: fields[0].col,
			Msg: fmt.Sprintf("names entry for entity %d is missing a name", id)})
		return NameAssignment{}, errs, false
	}
	return NameAssignment{EntityID: id, Name: fields[1].text, Line: lineNo}, errs, true
}

// parseOrder parses one order line. It validates that the command is known, its
// argument count is in range, its argument fields are well-formed, and required
// province parameters are coordinates. It records the order even when arity is
// wrong so downstream sees it; ok is false only when the command word is
// unknown.
func parseOrder(fields []field, lineNo int) (Order, []Error, bool) {
	var errs []Error
	word := strings.ToLower(fields[0].text)
	cmd, known := commandTable[word]
	if !known {
		errs = append(errs, Error{Line: lineNo, Col: fields[0].col,
			Msg: fmt.Sprintf("unknown command %q", word)})
		return Order{}, errs, false
	}

	argFields := fields[1:]
	args := make([]string, len(argFields))
	for i, a := range argFields {
		args[i] = a.text
	}

	// Arity.
	if len(args) < cmd.minArgs || (cmd.maxArgs >= 0 && len(args) > cmd.maxArgs) {
		errs = append(errs, Error{Line: lineNo, Col: fields[0].col,
			Msg: arityMessage(word, cmd, len(args))})
	}

	// Well-formed fields. Track which positions are malformed so a required
	// province check does not double-report the same field.
	malformed := make([]bool, len(argFields))
	for i, a := range argFields {
		if !wellFormedArg(a) {
			malformed[i] = true
			errs = append(errs, Error{Line: lineNo, Col: a.col,
				Msg: fmt.Sprintf("malformed argument %q", a.text)})
		}
	}

	// Required province positions.
	for _, p := range cmd.reqProv {
		if p >= len(argFields) || malformed[p] {
			continue
		}
		if !isCoord(argFields[p].text) {
			errs = append(errs, Error{Line: lineNo, Col: argFields[p].col,
				Msg: fmt.Sprintf("%s needs a province like (3,4), got %q", word, argFields[p].text)})
		}
	}

	return Order{ID: cmd.id, Word: word, Args: args, Line: lineNo, Col: fields[0].col}, errs, true
}

// arityMessage renders a friendly argument-count error for a command.
func arityMessage(word string, cmd command, got int) string {
	noun := func(k int) string {
		if k == 1 {
			return "argument"
		}
		return "arguments"
	}
	var phrase string
	switch {
	case got < cmd.minArgs && cmd.minArgs == cmd.maxArgs:
		phrase = fmt.Sprintf("%d %s", cmd.minArgs, noun(cmd.minArgs))
	case got < cmd.minArgs:
		phrase = fmt.Sprintf("at least %d %s", cmd.minArgs, noun(cmd.minArgs))
	case cmd.minArgs == cmd.maxArgs:
		phrase = fmt.Sprintf("%d %s", cmd.maxArgs, noun(cmd.maxArgs))
	default:
		phrase = fmt.Sprintf("at most %d %s", cmd.maxArgs, noun(cmd.maxArgs))
	}
	return fmt.Sprintf("%s expects %s, got %d", word, phrase, got)
}

// wellFormedArg reports whether an argument field is syntactically valid: a
// quoted text field, a canonical province coordinate, or a token that is not a
// broken coordinate. A field that looks like a coordinate attempt — it contains
// a parenthesis or comma — but is not a canonical coordinate is malformed.
func wellFormedArg(a field) bool {
	if a.quoted {
		return true
	}
	if isCoord(a.text) {
		return true
	}
	if strings.ContainsAny(a.text, "(),") {
		return false
	}
	return true
}

// isCoord reports whether s is a province coordinate in the canonical compact
// form "(q,r)" — no spaces, no redundant sign or padding.
func isCoord(s string) bool {
	var q, r int
	if n, err := fmt.Sscanf(s, "(%d,%d)", &q, &r); err != nil || n != 2 {
		return false
	}
	return fmt.Sprintf("(%d,%d)", q, r) == s
}
