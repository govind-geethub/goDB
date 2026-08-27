package main

import "log"

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

	if GetNumKeys(page0) == 0 {
		SetNodeType(page0, NodeTypeLeaf)
		_ = pager.WritePage(0, page0)
	}

	StartREPL(pager)
}
