package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "plugin":
		runPlugin()
	case "release":
		runRelease()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: go run . <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  plugin   Check and update plugin store versions")
	fmt.Println("  release  Create a new release from CHANGELOG.md")
}
