package main

import "github.com/thoscut/scanflow/client/internal/cli"

var version = "dev"

func main() {
	cli.Execute(version)
}
