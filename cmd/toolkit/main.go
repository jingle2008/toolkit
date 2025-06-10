/*
Package main is the entry point for the toolkit CLI application.
*/
package main

import "github.com/jingle2008/toolkit/internal/cli"

var version = "dev" // set via -ldflags "-X main.version=$(git describe --tags)"

func main() {
	cli.Execute(version)
}
