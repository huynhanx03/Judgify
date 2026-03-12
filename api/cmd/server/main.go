package main

import (
	"log"

	"github.com/huynhanx03/judgify/internal/infrastructure"
)

func main() {
	if err := infrastructure.Run(); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}
