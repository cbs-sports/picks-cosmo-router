package main

import (
	routercmd "github.com/wundergraph/cosmo/router/cmd"
	_ "github.com/wundergraph/cosmo/router/cmd/cbs-sports/auth"
	_ "github.com/wundergraph/cosmo/router/cmd/cbs-sports/cache-control"
)

func main() {
	routercmd.Main()
}
