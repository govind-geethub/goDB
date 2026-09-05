package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func StartREPL(pool *BufferPool) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("============================================")
	fmt.Println(" Custom B+ Tree Storage Engine REPL")
	fmt.Println(" Commands:")
	fmt.Println("   insert <id> <username> <email>")
	fmt.Println("   select <id>")
	fmt.Println("   scan <start_id> <end_id>")
	fmt.Println("   exit")
	fmt.Println("============================================")

	for {
		fmt.Print("db > ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		handleCommand(pool, line)
	}
}

func handleCommand(pool *BufferPool, input string) {
	parts := strings.Fields(input)
	command := strings.ToLower(parts[0])

	rootID, err := GetRootPageID(pool)
	if err != nil {
		fmt.Printf("Error resolving root: %v\n", err)
		return
	}

	switch command {
	case "insert":
		if len(parts) < 4 {
			fmt.Println("Usage: insert <id> <username> <email>")
			return
		}
		id, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			fmt.Println("Invalid ID")
			return
		}

		row := Row{
			ID:       uint32(id),
			UserName: parts[2],
			Email:    parts[3],
		}

		leafID, err := FindLeafPage(pool, rootID, row.ID)
		if err != nil {
			fmt.Printf("Error locating leaf: %v\n", err)
			return
		}

		leafPage, err := pool.FetchPage(leafID)
		if err != nil {
			fmt.Printf("Error fetching page: %v\n", err)
			return
		}

		err = LeafNodeInsertOrSplit(pool, leafID, leafPage, &row)
		if err != nil {
			fmt.Printf("Insert failed: %v\n", err)
		} else {
			fmt.Println("Executed.")
		}

	case "select":
		if len(parts) < 2 {
			fmt.Println("Usage: select <id>")
			return
		}
		id, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			fmt.Println("Invalid ID")
			return
		}

		row, err := BTreeSearch(pool, rootID, uint32(id))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("ID: %d | Username: %s | Email: %s\n", row.ID, row.UserName, row.Email)
		}

	case "scan":
		if len(parts) < 3 {
			fmt.Println("Usage: scan <start_id> <end_id>")
			return
		}
		start, err1 := strconv.ParseUint(parts[1], 10, 32)
		end, err2 := strconv.ParseUint(parts[2], 10, 32)
		if err1 != nil || err2 != nil {
			fmt.Println("Invalid range bounds")
			return
		}

		rows, err := BTreeScanRange(pool, rootID, uint32(start), uint32(end))
		if err != nil {
			fmt.Printf("Scan error: %v\n", err)
			return
		}

		fmt.Printf("Found %d rows:\n", len(rows))
		for _, r := range rows {
			fmt.Printf("  ID: %d | Username: %s | Email: %s\n", r.ID, r.UserName, r.Email)
		}

	default:
		fmt.Printf("Unrecognized command '%s'\n", command)
	}
}
