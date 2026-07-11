// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package orders

import "testing"

// $ go test ./internal/orders/ -run=Fuzz -fuzz=FuzzParse -fuzztime=30s
//
//	fuzz: elapsed: 0s, gathering baseline coverage: 0/2 completed
//	fuzz: elapsed: 0s, gathering baseline coverage: 2/2 completed, now fuzzing with 10 workers
//	fuzz: elapsed: 3s, execs: 783072 (260946/sec), new interesting: 195 (total: 197)
//	fuzz: elapsed: 6s, execs: 2129082 (448776/sec), new interesting: 289 (total: 291)
//	fuzz: elapsed: 9s, execs: 3386854 (419237/sec), new interesting: 320 (total: 322)
//	fuzz: elapsed: 12s, execs: 4888215 (500400/sec), new interesting: 345 (total: 347)
//	fuzz: elapsed: 15s, execs: 6184410 (432053/sec), new interesting: 368 (total: 370)
//	fuzz: elapsed: 18s, execs: 7555000 (456918/sec), new interesting: 384 (total: 386)
//	fuzz: elapsed: 21s, execs: 8936245 (460407/sec), new interesting: 388 (total: 390)
//	fuzz: elapsed: 24s, execs: 10322391 (461956/sec), new interesting: 401 (total: 403)
//	fuzz: elapsed: 27s, execs: 11707543 (461796/sec), new interesting: 427 (total: 429)
//	fuzz: elapsed: 30s, execs: 13050165 (447595/sec), new interesting: 446 (total: 448)
//	fuzz: elapsed: 30s, execs: 13050165 (0/sec), new interesting: 446 (total: 448)
//	PASS
//	ok  	github.com/mdhender/tpty/internal/orders	30.646s
func FuzzParse(f *testing.F) {
	f.Add("\"g\" 1 \"p\"\nentity 1, \"x\"\n    move 1 2\n")
	f.Add("entity notanid,\npillage (3,")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(s) // must not panic
	})
}
