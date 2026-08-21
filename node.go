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

// GetNodeType returns the type of node (Internal or Leaf)
func GetNodeType(page *Page) byte {
	return page[HeaderTypeOffset]
}

// SetNodeType sets the node type byte of a page
func SetNodeType(page *Page, nodeType byte) {
	page[HeaderTypeOffset] = nodeType
}

// GetNumKeys extracts 16-bit key counter from page header
func GetNumKeys(page *Page) uint16 {
	return binary.LittleEndian.Uint16(page[HeaderKeysOffset : HeaderKeysOffset+2])
}

// SetNumKeys updates the 16-bit key counter in the page header
func SetNumKeys(page *Page, numKeys uint16) {
	binary.LittleEndian.PutUint16(page[HeaderKeysOffset:HeaderKeysOffset+2], numKeys)
}

// LeafNodeSearch finds slot index via binary search
func LeafNodeSearch(page *Page, key uint32) (uint16, bool) {
	numKeys := GetNumKeys(page)
	if numKeys == 0 {
		return 0, false
	}

	var low uint16 = 0
	var high uint16 = numKeys

	for low < high {
		mid := (low + high) / 2

		// calc byte offset for mid inside the page
		offset := NodeHeaderSize + (uint32(mid) * RowSize)

		// extract 4-byte ID
		rowID := binary.LittleEndian.Uint32(page[offset : offset+4])

		if rowID == key {
			return mid, true
		} else if rowID < key {
			low = mid + 1
		} else {
			high = mid
		}
	}

	return low, false
}

func LeafNodeInsert(page *Page, row *Row) error {
	numKeys := GetNumKeys(page)
	maxKeys := uint16((PageSize - NodeHeaderSize) / RowSize) // (4096 - 8) / 291 = 14 rows

	if numKeys >= maxKeys {
		return fmt.Errorf("page full: cannot insert into Leaf Node")
	}

	slot, found := LeafNodeSearch(page, row.ID)
	if found {
		return fmt.Errorf("Duplicate key error: primary key %d already exists", row.ID)
	}

	rowData, err := row.Serialize()
	if err != nil {
		return fmt.Errorf("Failed to serialize row: %w", err)
	}

	// Shift existing rows to right in a single copy call
	if slot < numKeys {
		srcOffset := NodeHeaderSize + (uint32(slot) * RowSize)
		dstOffset := NodeHeaderSize + (uint32(slot+1) * RowSize)
		bytesToMove := uint32(numKeys-slot) * RowSize
		copy(page[dstOffset:dstOffset+bytesToMove], page[srcOffset:srcOffset+bytesToMove])
	}

	// Copy new row into designated slot
	targetOffset := NodeHeaderSize + (uint32(slot) * RowSize)
	copy(page[targetOffset:targetOffset+RowSize], rowData[:])

	SetNumKeys(page, numKeys+1)
	return nil
}

// LeafNodeInsert places a row into a leaf page. If the page is full, it triggers LeafNodeSplit.
func LeafNodeInsertOrSplit(pager *Pager, pageID uint32, page *Page, row *Row) error {
	numKeys := GetNumKeys(page)
	maxKeys := uint16((PageSize - NodeHeaderSize) / RowSize) // 14

	if numKeys >= maxKeys {
		// LeafNodeSplit internal logic writes both oldPage and newPage to disk
		_, err := LeafNodeSplit(pager, pageID, page, row)
		return err
	}

	return LeafNodeInsert(page, row)
}

func GetNextPage(page *Page) uint32 {
	return binary.LittleEndian.Uint32(page[HeaderNextOffset : HeaderNextOffset+4])
}

func SetNextPage(page *Page, nextPageID uint32) {
	binary.LittleEndian.PutUint32(page[HeaderNextOffset:HeaderNextOffset+4], nextPageID)
}

// LeafNodeSplit splits a full leaf node into two 50/50 nodes
func LeafNodeSplit(pager *Pager, oldPageID uint32, oldPage *Page, newRow *Row) (uint32, error) {
	numKeys := GetNumKeys(oldPage)

	// Gather existing rows
	rows := make([]Row, 0, numKeys+1)
	for i := uint16(0); i < numKeys; i++ {
		offset := NodeHeaderSize + (uint32(i) * RowSize)
		r, err := Deserialize(oldPage[offset : offset+RowSize])
		if err != nil {
			return 0, fmt.Errorf("Failed to deserialize row during splitting: %w", err)
		}
		rows = append(rows, *r)
	}

	// Insert 15th row in sorted order
	inserted := false
	for i, r := range rows {
		if newRow.ID == r.ID {
			return 0, fmt.Errorf("Duplicate key error: primary key %d already exists", newRow.ID)
		}

		if newRow.ID < r.ID {
			rows = append(rows[:i], append([]Row{*newRow}, rows[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		rows = append(rows, *newRow)
	}

	// Allocate new page for right sibling
	newPageID := uint32(pager.fileSize / PageSize)
	newPage, err := pager.ReadPage(newPageID)
	if err != nil {
		return 0, fmt.Errorf("Failed to allocate new page for split: %w", err)
	}

	SetNodeType(newPage, NodeTypeLeaf)
	SetNumKeys(newPage, 0)

	// Linked list pointers for leaf nodes
	SetNextPage(newPage, GetNextPage(oldPage))
	SetNextPage(oldPage, newPageID)

	// Split 15 rows: 7 in old Page, 8 in new Page
	mid := 7

	// Clear payload memory of old page
	clear(oldPage[NodeHeaderSize:])
	SetNumKeys(oldPage, 0)

	// Populate left half
	for i := 0; i < mid; i++ {
		if err := LeafNodeInsert(oldPage, &rows[i]); err != nil {
			return 0, fmt.Errorf("Failed writing to old Page during split: %w", err)
		}
	}

	// Populate right half
	for i := mid; i < len(rows); i++ {
		if err := LeafNodeInsert(newPage, &rows[i]); err != nil {
			return 0, fmt.Errorf("Failed writing to new Page during split: %w", err)
		}
	}

	// Write both back to disk
	if err := pager.WritePage(oldPageID, oldPage); err != nil {
		return 0, fmt.Errorf("Failed to write old page: %w", err)
	}

	if err := pager.WritePage(newPageID, newPage); err != nil {
		return 0, fmt.Errorf("Failed to write new page: %w", err)
	}

	return newPageID, nil
}

// internal load layouts
// header: 8 bytes(type, reserved, numKeys, nextPage)
const (
	InternalNodeKeySize   = 4
	InternalNodeChildSize = 4
)

// internalNodeChild returns the page ID of child pointer at index childNum
func InternalNodeChild(page *Page, childNum uint16) uint32 {
	numKeys := GetNumKeys(page)
	if childNum > numKeys {
		panic(fmt.Sprintf("childNum %d out of bounds for numKeys %d", childNum, numKeys))
	}

	// children array starts right after the 8 byte header
	offset := NodeHeaderSize + (uint32(childNum) * InternalNodeChildSize)
	return binary.LittleEndian.Uint32(page[offset : offset+InternalNodeChildSize])
}

// SetInternalNodeChild updates child pointer at index childNum
func SetInternalNodeChild(page *Page, childNum uint16, childPageID uint32) {
	offset := NodeHeaderSize + (uint32(childNum) * InternalNodeChildSize)
	binary.LittleEndian.PutUint32(page[offset:offset+InternalNodeChildSize], childPageID)
}

// internalNodeKey return the separator key at index keyNum
func InternalNodeKey(page *Page, keyNum uint16) uint32 {
	numKeys := GetNumKeys(page)

	// Keys array starts after header + children pointers
	// max children = max keys + 1
	childrenOffset := NodeHeaderSize + (uint32(numKeys+1) * InternalNodeChildSize)
	keyOffset := childrenOffset + (uint32(keyNum) * InternalNodeKeySize)
	return binary.LittleEndian.Uint32(page[keyOffset : keyOffset+InternalNodeKeySize])
}

// InternalNodeFindChildSize returns the page ID that contains the taregt key
func InternalNodeFindChildSize(page *Page, key uint32) uint32 {
	numKeys := GetNumKeys(page)

	// binary search through the internal node keys
	var low uint16 = 0
	var high uint16 = numKeys

	for low < high {
		mid := (low + high) / 2
		k := InternalNodeKey(page, mid)

		if k <= key {
			low = mid + 1
		} else {
			high = mid
		}
	}

	// low index points to the required target
	return InternalNodeChild(page, low)
}
