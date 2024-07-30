package main

import (
	routercmd "github.com/wundergraph/cosmo/router/cmd"
	_ "github.com/wundergraph/cosmo/router/cmd/custom-cache-control/module"
)

func main() {
	routercmd.Main()
}
