package main

import (
	"github.com/NipulM/oisbase/cmd"
)

var version = "dev"

func main() {
	cmd.Execute(version)
}