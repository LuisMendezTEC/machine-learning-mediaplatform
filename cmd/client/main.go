package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	coordinatorURL := os.Getenv("COORDINATOR_URL")
	if coordinatorURL == "" {
		coordinatorURL = "http://localhost:8080"

	}
	fmt.Printf("[client] pointing to coordinator at %s\n", coordinatorURL)
	log.Println("[client] ready — job submission not yet implemented")
}
