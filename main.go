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
		log.Fatalf("Failed to read Page 0: %v", err)
	}

	// Set Page 0 as a Leaf Node
	SetNodeType(page0, NodeTypeLeaf)

	// Insert rows OUT OF ORDER (30, 10, 20)
	rows := []Row{
		{ID: 30, UserName: "Charlie", Email: "charlie@example.com"},
		{ID: 10, UserName: "Alice", Email: "alice@example.com"},
		{ID: 20, UserName: "Bob", Email: "bob@example.com"},
	}

	for _, r := range rows {
		err := LeafNodeInsert(page0, &r)
		if err != nil {
			log.Fatalf("Insert failed for ID %d: %v", r.ID, err)
		}
	}

	// Write Page 0 to disk
	err = pager.WritePage(0, page0)
	if err != nil {
		log.Fatalf("Failed to write Page 0: %v", err)
	}

	// Read Page 0 back and print items to prove they were sorted automatically!
	numKeys := GetNumKeys(page0)
	fmt.Printf("Page 0 NodeType: %d, Total Keys Stored: %d\n", GetNodeType(page0), numKeys)

	for i := uint16(0); i < numKeys; i++ {
		offset := NodeHeaderSize + (uint32(i) * RowSize)
		r, _ := Deserialize(page0[offset : offset+RowSize])
		fmt.Printf("Slot %d -> ID: %d, User: %s\n", i, r.ID, r.UserName)
	}
}
