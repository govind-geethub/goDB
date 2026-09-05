package main

import (
	"log"
)

func main() {
	pager, err := NewPager("db.bin")
	if err != nil {
		log.Fatalf("Failed to initialize pager: %v", err)
	}
	defer pager.Close()

	// Create BufferPool with a capacity of 10 pages
	pool := NewBufferPool(10, pager)
	defer func() {
		if err := pool.FlushAll(); err != nil {
			log.Printf("Error flushing buffer pool on exit: %v", err)
		}
	}()

	// Ensure Page 0 is properly initialized as a Leaf node
	page0, err := pool.FetchPage(0)
	if err != nil {
		log.Fatalf("Failed to fetch page 0: %v", err)
	}

	if GetNumKeys(page0) == 0 && GetNodeType(page0) == 0 {
		SetNodeType(page0, NodeTypeLeaf)
		pool.MarkDirty(0)
	}

	// Launch REPL
	StartREPL(pool)
}
