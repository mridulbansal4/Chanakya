package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

func main() {
	reModified := regexp.MustCompile(`(?i)([^.;\n]{4,200}?)\s+stands?\s+(?:modified|amended)`)
	text := strings.Repeat("This is a random sentence that does not match. ", 40000) // ~2MB
	start := time.Now()
	res := reModified.FindAllStringSubmatch(text, -1)
	fmt.Printf("Matches: %d, Time: %v\n", len(res), time.Since(start))
}
