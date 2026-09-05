//go:build !windows

// The sticky window hook is a Windows-only mechanism; this stub keeps the package
// buildable on other platforms.
package main

func main() {}
