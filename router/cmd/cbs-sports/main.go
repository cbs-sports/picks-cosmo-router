package main

import (
	routercmd "github.com/wundergraph/cosmo/router/cmd"
	_ "github.com/wundergraph/cosmo/router/cmd/cbs-sports/auth"
)

func main() {
	routercmd.Main()
}
