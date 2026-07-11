// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Package main implements an order parser for command line testing
// and validation.
//
// Usage: go run ./cmd/parse < someorders.txt
package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/mdhender/tpty/internal/orders"
)

func main() {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("read: %v\n", err)
	}
	f, errs := orders.Parse(string(src))

	fmt.Printf("opening: %+v\n", f.Opening)

	fmt.Printf("blocks recovered: %d\n", len(f.Entities))
	for _, b := range f.Entities {
		fmt.Printf("  entity %d %q: %d orders, %d names\n",
			b.EntityID, b.Name, len(b.Orders), len(b.Names))
	}

	fmt.Printf("%d errors:\n", len(errs))
	for _, e := range errs {
		fmt.Printf("  %s\n", e.Error())
	}
}
