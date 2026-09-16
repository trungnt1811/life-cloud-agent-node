package main

import (
	"log"

	app "github.com/lifenetwork-ai/life-cloud-agent-node/cmd/app"
	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
)

func main() {
	config, err := conf.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	if err := app.RunApp(config); err != nil {
		log.Fatalf("Application stopped with error: %v", err)
	}
}
