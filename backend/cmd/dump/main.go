package main

import (
	"context"
	"fmt"
	"log"

	"chanakya/internal/store"
)

func main() {
	st, err := store.Open(context.Background(), "../../chanakya.db")
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	runs, err := st.ListIngestRuns(context.Background(), 100)
	if err != nil {
		log.Fatal(err)
	}
	
	count := 0
	for _, r := range runs {
		fmt.Printf("Run ID: %s, Filename: %s, State: %s\n", r.ID, r.Filename, r.State)
		if err := st.DiscardIngestRun(context.Background(), r.ID); err != nil {
			log.Printf("Error discarding: %v\n", err)
		} else {
			count++
		}
	}
	fmt.Printf("Discarded %d runs in preview state.\n", count)
}
