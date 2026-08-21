package main

import (
	"fmt"
	"log"
)

func main() {
	pager, err := NewPager("test_btree.db")
	if err != nil {
		log.Fatalf("Failed to initialize pager: %v", err)
	}
	defer pager.Close()

	page0, err := pager.ReadPage(0)
	if err != nil {
		log.Fatalf("Failed to read page 0: %v", err)
	}

	if GetNumKeys(page0) == 0 {
		SetNodeType(page0, NodeTypeLeaf)
	}

	// Insert 15 rows to trigger a split on the 15th key
	fmt.Println("Inserting 15 rows into Database...")
	for i := uint32(1); i <= 15; i++ {
		row := Row{
			ID:       i,
			UserName: fmt.Sprintf("User %d", i),
			Email:    fmt.Sprintf("user_%d@gmail.com", i),
		}

		p0, err := pager.ReadPage(0)
		if err != nil {
			log.Fatalf("Failed to read page 0 for row %d: %v", i, err)
		}

		// LeafNodeInsertOrSplit updates disk & reloads p0 if a split occurs
		err = LeafNodeInsertOrSplit(pager, 0, p0, &row)
		if err != nil {
			log.Fatalf("Failed on row %d: %v", i, err)
		}

		// If no split occurred, write the updated page back to disk
		if GetNumKeys(p0) < 14 {
			_ = pager.WritePage(0, p0)
		}
	}

	// Create Page 2 as the Internal Root Node
	// Left child: Page 0 (IDs 1–7)
	// Right child: Page 1 (IDs 8–15)
	rootPageID := uint32(2)
	splitKey := uint32(8)

	err = CreateRootNode(pager, rootPageID, 0, 1, splitKey)
	if err != nil {
		log.Fatalf("Failed to create root node: %v", err)
	}
	fmt.Println("Created Internal Root Node (Page 2) -> Left: Page 0 | Split Key: 8 | Right: Page 1")

	// Multilevel Tree traversal check
	searchKeys := []uint32{4, 12, 15}
	fmt.Println("\n--- Testing B-Tree Search from Root (Page 2) ---")
	for _, k := range searchKeys {
		row, err := BTreeSearch(pager, rootPageID, k)
		if err != nil {
			fmt.Printf("Search key %d: Error -> %v\n", k, err)
		} else {
			fmt.Printf("Search key %d: Found -> ID: %d, User: %s, Email: %s\n", k, row.ID, row.UserName, row.Email)
		}
	}
}
