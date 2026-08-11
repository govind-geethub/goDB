package main

import (
	"fmt"
	"log"
)

func main() {
	// 1. Initialize or open your pager database file
	pager, err := NewPager("db.bin")
	if err != nil {
		log.Fatalf("Failed to initialize pager: %v", err)
	}
	defer pager.Close()

	// 2. Fetch or initialize Page 0 (Leaf Node)
	pageID := uint32(0)
	page, err := pager.ReadPage(pageID)
	if err != nil {
		log.Fatalf("Failed to read page %d: %v", pageID, err)
	}

	// Initialize header if it's a fresh page
	if GetNodeType(page) == 0 {
		SetNodeType(page, NodeTypeLeaf)
		SetNumKeys(page, 0)
		SetNextPage(page, 0)
	}

	// 3. Insert rows into the leaf node until full
	// (Max capacity = 14 rows for 4096-byte page with 291-byte rows)
	for i := uint32(1); i <= 14; i++ {
		row := &Row{
			ID:       i,
			UserName: fmt.Sprintf("user%d", i),
			Email:    fmt.Sprintf("user%d@example.com", i),
		}

		if err := LeafNodeInsert(page, row); err != nil {
			log.Fatalf("Failed to insert row ID %d: %v", i, err)
		}
		fmt.Printf("Inserted Row ID %d into Page %d (Total Keys: %d)\n", row.ID, pageID, GetNumKeys(page))
	}

	// Save full page back to disk
	if err := pager.WritePage(pageID, page); err != nil {
		log.Fatalf("Failed to write page: %v", err)
	}

	// 4. Trigger LeafNodeSplit when attempting to insert the 15th row
	newRow := &Row{
		ID:       15,
		UserName: "user15",
		Email:    "user15@example.com",
	}

	fmt.Println("\nAttempting 15th insertion (Page Full) -> Splitting Page...")

	newPageID, err := LeafNodeSplit(pager, pageID, page, newRow)
	if err != nil {
		log.Fatalf("Failed to split page: %v", err)
	}

	// 5. Verify split results
	newPage, err := pager.ReadPage(newPageID)
	if err != nil {
		log.Fatalf("Failed to read newly allocated split page: %v", err)
	}

	fmt.Printf("\n--- Split Complete ---\n")
	fmt.Printf("Old Page ID %d Keys: %d (Next Page -> %d)\n", pageID, GetNumKeys(page), GetNextPage(page))
	fmt.Printf("New Page ID %d Keys: %d (Next Page -> %d)\n", newPageID, GetNumKeys(newPage), GetNextPage(newPage))
}
