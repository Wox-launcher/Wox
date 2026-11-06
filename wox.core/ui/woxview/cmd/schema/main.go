// Command schema writes the current WoxView JSON Schema to standard output.
package main

import (
	"os"

	"wox/ui/woxview"
)

func main() {
	if _, err := os.Stdout.Write(woxview.JSONSchema()); err != nil {
		panic(err)
	}
}
