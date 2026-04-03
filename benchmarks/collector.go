package main

import (
	"encoding/csv"
	"fmt"
	"os"
)

func main() {
	file, _ := os.Open("benchmarks/raw_results.csv")
	defer file.Close()

	records, _ := csv.NewReader(file).ReadAll()

	fmt.Printf("\n%-15s | %-10s | %-10s\n", "Task", "Duration", "Verify")
	fmt.Println("-------------------------------------------")
	for _, row := range records {
		fmt.Printf("%-15s | %-10s | %-10s\n", row[0], row[1]+"ms", row[2])
	}
}
