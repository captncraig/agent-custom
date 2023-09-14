package main

import (
	// this could use github.com/grafana/agent/cmd/internal/flowmode if it were not internal
	"github.com/captncraig/agent-custom/flowmode"

	// import custom components
	_ "github.com/captncraig/agent-custom/component/mdns"
)

func main() {
	flowmode.Run()
}
