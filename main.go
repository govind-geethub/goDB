package main

import (
	"fmt"
	"log"
)

// GetRootPageID walks parent pointers up to the top root node
func GetRootPageID(pager *Pager) (uint32, error) {
	p0, err := pager.ReadPage(0)
	if err != nil {
		return 0, err
	}

	rootID := GetParentPageID(p0)
	if rootID == 0 {
		return 0, nil
	}

	for {
		parentPage, err := pager.ReadPage(rootID)
		if err != nil {
			return rootID, nil
		}
		parentOfParent := GetParentPageID(parentPage)
		if parentOfParent == 0 {
			break
		}
		rootID = parentOfParent
	}

	return rootID, nil
}

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

	// Insert 50 rows, traversing down from root to target leaf on each step
	fmt.Println("Inserting 50 rows into database...")
	for i := uint32(1); i <= 50; i++ {
		row := Row{
			ID:       i,
			UserName: fmt.Sprintf("User_%d", i),
			Email:    fmt.Sprintf("user_%d@gmail.com", i),
		}

		// 1. Resolve current tree root dynamically
		rootID, err := GetRootPageID(pager)
		if err != nil {
			log.Fatalf("Failed to retrieve root page ID: %v", err)
		}

		// 2. Find correct leaf page target via B-Tree traversal
		leafID, err := FindLeafPage(pager, rootID, row.ID)
		if err != nil {
			log.Fatalf("Failed to locate target leaf page for key %d: %v", row.ID, err)
		}

		// 3. Read leaf page and execute insertion/split
		leafPage, err := pager.ReadPage(leafID)
		if err != nil {
			log.Fatalf("Failed to read leaf page %d: %v", leafID, err)
		}

		err = LeafNodeInsertOrSplit(pager, leafID, leafPage, &row)
		if err != nil {
			log.Fatalf("Failed on row %d: %v", i, err)
		}
	}

	// Fetch final top-level Root ID
	rootID, err := GetRootPageID(pager)
	if err != nil {
		log.Fatalf("Failed to retrieve root page ID: %v", err)
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
