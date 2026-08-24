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
		_ = pager.WritePage(0, page0)
	}

	// Insert 50 rows to force multiple internal node cascading splits
	fmt.Println("Inserting 50 rows into database...")
	for i := uint32(1); i <= 50; i++ {
		row := Row{
			ID:       i,
			UserName: fmt.Sprintf("User_%d", i),
			Email:    fmt.Sprintf("user_%d@gmail.com", i),
		}

		p0, err := pager.ReadPage(0)
		if err != nil {
			log.Fatalf("Failed to read page 0: %v", err)
		}

		err = LeafNodeInsertOrSplit(pager, 0, p0, &row)
		if err != nil {
			log.Fatalf("Failed on row %d: %v", i, err)
		}
	}

	p0, err := pager.ReadPage(0)
	if err != nil {
		log.Fatalf("Failed to read page 0: %v", err)
	}

	// Walk parent links to find top Root ID dynamically
	rootID := GetParentPageID(p0)
	for {
		parentPage, err := pager.ReadPage(rootID)
		if err != nil || GetParentPageID(parentPage) == 0 {
			break
		}
		rootID = GetParentPageID(parentPage)
	}

	fmt.Printf("Tree successfully grew! Dynamic Root is now at Page %d\n\n", rootID)

	searchKeys := []uint32{1, 15, 25, 42, 50}
	fmt.Println("--- Testing Multi-Level B-Tree Search ---")
	for _, k := range searchKeys {
		row, err := BTreeSearch(pager, rootID, k)
		if err != nil {
			fmt.Printf("Search Key %d: Error -> %v\n", k, err)
		} else {
			fmt.Printf("Search Key %d: Found -> ID: %d, User: %s, Email: %s\n", k, row.ID, row.UserName, row.Email)
		}
	}
}
