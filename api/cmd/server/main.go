package main

import (
	"fmt"
	"os"

	"github.com/huynhanx03/judgify/internal/infrastructure"
)

func main() {
	if err := infrastructure.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "server failed to start: %v\n", err)
		os.Exit(1)
	}
}
