package main

import (
	"encoding/binary"
	"fmt"
)

// Node Types
const (
	NodeTypeInternal = 1
	NodeTypeLeaf     = 2
)

// Header offsets inside the Page
const (
	HeaderTypeOffset = 0
	HeaderRootOffset = 1
	HeaderKeysOffset = 2
	HeaderNextOffset = 4
	NodeHeaderSize   = 8 // total bytes reserved for page metadata
)

// node is Internal or, Leaf
func GetNodeType(page *Page) byte {
	return page[HeaderTypeOffset]
}

// sets the first byte of a page
func SetNodeType(page *Page, nodeType byte) {
	page[HeaderTypeOffset] = nodeType
}

// extracts 16 bit key counter from page header
func GetNumKeys(page *Page) uint16 {
	return binary.LittleEndian.Uint16(page[HeaderKeysOffset : HeaderKeysOffset+2])
}

// updates the 16 bit key counter in the page header
func SetNumKeys(page *Page, numKeys uint16) {
	binary.LittleEndian.PutUint16(page[HeaderKeysOffset:HeaderKeysOffset+2], numKeys)
}

// where the row belongs binary search
func LeafNodeSearch(page *Page, key uint32) (uint16, bool) {
	numKeys := GetNumKeys(page)
	if numKeys == 0 {
		return 0, false
	}

	var low uint16 = 0
	var high uint16 = numKeys

	for low < high {
		mid := (low + high) / 2

		// calc byte offset fo mid inside the page
		// skip the 8 byte metadata + (mid * 291)
		offset := NodeHeaderSize + (uint32(mid) * RowSize)

		// extract 4 byte ID
		rowID := binary.LittleEndian.Uint32(page[offset : offset+4])

		if rowID == key {
			return mid, true
		} else if rowID < key {
			low = mid + 1
		} else {
			high = mid
		}
	}

	// here the key will be inserted into the low index
	// false new row to be inserted
	return low, false
}

func LeafNodeInsert(page *Page, row *Row) error {
	numKeys := GetNumKeys(page)

	// calc max rows a page with 8 byte header holds
	maxKeys := uint16((PageSize - NodeHeaderSize) / RowSize) // (4096 - 8) / 291 = 14 rows

	if numKeys >= maxKeys {
		return fmt.Errorf("page full : cannot insert into Leaf Node")
	}

	// target slot through Binary Search
	slot, found := LeafNodeSearch(page, row.ID)
	if found {
		return fmt.Errorf("Duplicate key error: primary key %d already exists", row.ID)
	}

	// serialize row into 291 byte
	rowData, err := row.Serialize()
	if err != nil {
		return fmt.Errorf("Failed to serialize row: %w", err)
	}

	// shift existing rows to the right to amke a gap at "slot"
	for i := numKeys; i > slot; i-- {
		srcOffset := NodeHeaderSize + (uint32(i-1) * RowSize)
		dstOffset := NodeHeaderSize + (uint32(i) * RowSize)
		copy(page[dstOffset:dstOffset+RowSize], page[srcOffset:srcOffset+RowSize])
	}

	// copy the new row bytes into newly freed "slot"
	targetOffset := NodeHeaderSize + (uint32(slot) * RowSize)
	copy(page[targetOffset:targetOffset+RowSize], rowData[:])

	// increment and save key count in page header
	SetNumKeys(page, numKeys+1)

	return nil
}
