package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "./chanakya.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// List matching entries before deletion
	rows, err := db.Query("SELECT id, filename FROM ingest_run WHERE filename LIKE '%srujan%' OR filename LIKE '%resume%'")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, fn string
			if err := rows.Scan(&id, &fn); err == nil {
				fmt.Printf("Found ingest_run: %s (%s)\n", id, fn)
			}
		}
	} else {
		log.Printf("Query error: %v", err)
	}

	res1, err := db.Exec("DELETE FROM ingest_run WHERE filename LIKE '%srujan%' OR filename LIKE '%resume%'")
	if err != nil {
		log.Printf("Error deleting from ingest_run: %v", err)
	} else {
		n, _ := res1.RowsAffected()
		fmt.Printf("Deleted %d rows from ingest_run\n", n)
	}

	res2, err := db.Exec("DELETE FROM document_blob WHERE filename LIKE '%srujan%' OR filename LIKE '%resume%'")
	if err != nil {
		log.Printf("Error deleting from document_blob: %v", err)
	} else {
		n, _ := res2.RowsAffected()
		fmt.Printf("Deleted %d rows from document_blob\n", n)
	}
}
