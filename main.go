package main

import (
	"fmt"
	"log"
)

func main() {
	pager, err := NewPager("test_btree.db")
	if err != nil {
		log.Fatalf("Failed to initialize pager: %w", err)
	}
	defer pager.Close()

	page0, err := pager.ReadPage(0)
	if err != nil {
		log.Fatalf("failed to read page 0: %v", err)
	}

	// initalize page 0 as Leaf if its New (0 Keys)
	if GetNumKeys(page0) == 0 {
		SetNodeType(page0, NodeTypeLeaf)
	}

	// insert 15 rows (1 -> 15) to trigger page split on the 15th row
	fmt.Println("Inserting 15 rows into the database...")
	for i := uint32(1); i <= 15; i++ {
		row := Row{
			ID:       i,
			UserName: fmt.Sprintf("User_%d", i),
			Email:    fmt.Sprintf("user_%d@gmail.com", i),
		}

		// reload page 0 fresh
		p0, err := pager.ReadPage(0)
		if err != nil {
			log.Fatalf("Failed to read page 0: %v", err)
		}

		// insert or split
		err = LeafNodeInsertOrSplit(pager, 0, p0, &row)
		if err != nil {
			log.Fatalf("failed on row %d: %v", i, err)
		}

		// persist the changes back to disk
		err = pager.WritePage(0, p0)
		if err != nil {
			log.Fatalf("Failed to save page 0 state to disl: %v", err)
		}
	}

	// verify page 0 contents
	p0, _ := pager.ReadPage(0)
	keysP0 := GetNumKeys(p0)
	nextP0 := GetNextPage(p0)
	fmt.Printf("\n --- Page 0 (Keys: %d, Next Page: %d) --- \n", keysP0, nextP0)
	for i := uint16(0); i < keysP0; i++ {
		offset := NodeHeaderSize + (uint32(i) * RowSize)
		r, _ := Deserialize(p0[offset : offset+RowSize])
		fmt.Printf("Slot %d -> ID: %d, User: %s\n", i, r.ID, r.UserName)
	}

	// verify page 1 contents (new sibling)
	p1, _ := pager.ReadPage(1)
	keysP1 := GetNumKeys(p1)
	nextP1 := GetNextPage(p1)
	fmt.Printf("\n --- Page 1 (Keys: %d, Next Page: %d) ---\n", keysP1, nextP1)
	for i := uint16(0); i < keysP1; i++ {
		offset := NodeHeaderSize + (uint32(i) * RowSize)
		r, _ := Deserialize(p1[offset : offset+RowSize])
		fmt.Printf("Slot %d -> ID: %d, User: %s\n", i, r.ID, r.UserName)
	}
}
