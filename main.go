package main

import (
	"fmt"
	"log"
)

func main() {
	pager, err := NewPager("test_btree.db")
	if err != nil {
		log.Fatalf("Failed to initliaze pager: %v", err)
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

	// insert 15 rows to trigger split and root creation
	fmt.Println("Inserting 15 rows into database...")
	for i := uint32(1); i <= 15; i++ {
		row := Row{
			ID:       i,
			UserName: fmt.Sprintf("User_%d", i),
			Email:    fmt.Sprintf("user_%d_@gmail.com", i),
		}

		p0, err := pager.ReadPage(0)
		if err != nil {
			log.Fatalf("Failed to read page 0 for row %d: %v ", i, err)
		}

		err = LeafNodeInsertOrSplit(pager, 0, p0, &row)
		if err != nil {
			log.Fatalf("Failed on row %d: %v", i, err)
		}
	}

	// read root node ID from child
	p0, err := pager.ReadPage(0)
	if err != nil {
		log.Fatalf("Failed to read page 0: %v", err)
	}

	rootPageID := GetParentPageID(p0)
	fmt.Printf("Dynamic Root node created at page %d! \n", rootPageID)

	// multi level tree traversal
	searchKeys := []uint32{4, 8, 12, 15}
	fmt.Println("\n --- Testing dynamic B-Tree search ---")
	for _, k := range searchKeys {
		row, err := BTreeSearch(pager, rootPageID, k)
		if err != nil {
			fmt.Printf("Search key %d: Error -> %v\n", k, err)
		} else {
			fmt.Printf("Search Key %d: Found -> ID: %d, User: %s, Email: %s \n", k, row.ID, row.UserName, row.Email)
		}
	}
}
