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
	HeaderTypeOffset   = 0
	HeaderKeysOffset   = 1
	HeaderParentOffset = 3
	HeaderNextOffset   = 7
	NodeHeaderSize     = 11 // total bytes reserved for page metadata
)

func GetNodeType(page *Page) byte {
	return page[HeaderTypeOffset]
}

func SetNodeType(page *Page, nodeType byte) {
	page[HeaderTypeOffset] = nodeType
}

func GetNumKeys(page *Page) uint16 {
	return binary.LittleEndian.Uint16(page[HeaderKeysOffset : HeaderKeysOffset+2])
}

func SetNumKeys(page *Page, numKeys uint16) {
	binary.LittleEndian.PutUint16(page[HeaderKeysOffset:HeaderKeysOffset+2], numKeys)
}

func GetNextPage(page *Page) uint32 {
	return binary.LittleEndian.Uint32(page[HeaderNextOffset : HeaderNextOffset+4])
}

func SetNextPage(page *Page, nextPageID uint32) {
	binary.LittleEndian.PutUint32(page[HeaderNextOffset:HeaderNextOffset+4], nextPageID)
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
		offset := NodeHeaderSize + (uint32(mid) * RowSize)
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
	maxKeys := uint16((PageSize - NodeHeaderSize) / RowSize) // 14 rows

	if numKeys >= maxKeys {
		return fmt.Errorf("page full: cannot insert into Leaf Node")
	}

	slot, found := LeafNodeSearch(page, row.ID)
	if found {
		return fmt.Errorf("duplicate key error: primary key %d already exists", row.ID)
	}

	rowData, err := row.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize row: %w", err)
	}

	// Shift existing rows right
	if slot < numKeys {
		srcOffset := NodeHeaderSize + (uint32(slot) * RowSize)
		dstOffset := NodeHeaderSize + (uint32(slot+1) * RowSize)
		bytesToMove := uint32(numKeys-slot) * RowSize
		copy(page[dstOffset:dstOffset+bytesToMove], page[srcOffset:srcOffset+bytesToMove])
	}

	targetOffset := NodeHeaderSize + (uint32(slot) * RowSize)
	copy(page[targetOffset:targetOffset+RowSize], rowData[:])

	SetNumKeys(page, numKeys+1)
	return nil
}

func LeafNodeInsertOrSplit(pager *Pager, pageID uint32, page *Page, row *Row) error {
	numKeys := GetNumKeys(page)
	maxKeys := uint16((PageSize - NodeHeaderSize) / RowSize)

	if numKeys >= maxKeys {
		_, err := LeafNodeSplit(pager, pageID, page, row)
		if err != nil {
			return err
		}

		updatedPage, err := pager.ReadPage(pageID)
		if err != nil {
			return err
		}
		copy(page[:], updatedPage[:])
		return nil
	}

	err := LeafNodeInsert(page, row)
	if err != nil {
		return err
	}
	return pager.WritePage(pageID, page)
}

func LeafNodeSplit(pager *Pager, oldPageID uint32, oldPage *Page, newRow *Row) (uint32, error) {
	numKeys := GetNumKeys(oldPage)

	// Gather existing rows
	rows := make([]Row, 0, numKeys+1)
	for i := uint16(0); i < numKeys; i++ {
		offset := NodeHeaderSize + (uint32(i) * RowSize)
		r, err := Deserialize(oldPage[offset : offset+RowSize])
		if err != nil {
			return 0, fmt.Errorf("failed to deserialize row during splitting: %w", err)
		}
		rows = append(rows, *r)
	}

	// Insert new row into sorted array
	inserted := false
	for i, r := range rows {
		if newRow.ID == r.ID {
			return 0, fmt.Errorf("duplicate key error: primary key %d already exists", newRow.ID)
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

	// Allocate new right page
	newPageID := uint32(pager.fileSize / PageSize)
	newPage, err := pager.ReadPage(newPageID)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate new page for split: %w", err)
	}

	// Setup new page metadata
	SetNodeType(newPage, NodeTypeLeaf)
	SetNumKeys(newPage, 0)
	SetNextPage(newPage, GetNextPage(oldPage))

	// Link old page to new page
	SetNextPage(oldPage, newPageID)

	// Clear old page payload (keep header bytes 0–7 intact)
	for i := NodeHeaderSize; i < PageSize; i++ {
		oldPage[i] = 0
	}
	SetNodeType(oldPage, NodeTypeLeaf)
	SetNumKeys(oldPage, 0)

	// Split 15 rows: 7 in left page, 8 in right page
	mid := 7

	for i := 0; i < mid; i++ {
		if err := LeafNodeInsert(oldPage, &rows[i]); err != nil {
			return 0, fmt.Errorf("failed writing to old Page during split: %w", err)
		}
	}

	for i := mid; i < len(rows); i++ {
		if err := LeafNodeInsert(newPage, &rows[i]); err != nil {
			return 0, fmt.Errorf("failed writing to new Page during split: %w", err)
		}
	}

	if err := pager.WritePage(oldPageID, oldPage); err != nil {
		return 0, fmt.Errorf("failed to write old page: %w", err)
	}

	if err := pager.WritePage(newPageID, newPage); err != nil {
		return 0, fmt.Errorf("failed to write new page: %w", err)
	}

	splitKey := rows[mid].ID
	err = InsertIntoParent(pager, oldPageID, newPageID, splitKey)
	if err != nil {
		return 0, fmt.Errorf("failed to insert key %d into parent: %w", splitKey, err)
	}
	return newPageID, nil
}

// Internal node layout configurations
const (
	InternalNodeKeySize   = 4
	InternalNodeChildSize = 4
	MaxInternalChildren   = 3
)

func InternalNodeChild(page *Page, childNum uint16) uint32 {
	offset := NodeHeaderSize + (uint32(childNum) * InternalNodeChildSize)
	return binary.LittleEndian.Uint32(page[offset : offset+InternalNodeChildSize])
}

func SetInternalNodeChild(page *Page, childNum uint16, childPageID uint32) {
	offset := NodeHeaderSize + (uint32(childNum) * InternalNodeChildSize)
	binary.LittleEndian.PutUint32(page[offset:offset+InternalNodeChildSize], childPageID)
}

func InternalNodeKey(page *Page, keyNum uint16) uint32 {
	childrenSectionSize := uint32(MaxInternalChildren) * InternalNodeChildSize
	keyOffset := NodeHeaderSize + childrenSectionSize + (uint32(keyNum) * InternalNodeKeySize)
	return binary.LittleEndian.Uint32(page[keyOffset : keyOffset+InternalNodeKeySize])
}

func SetInternalNodeKey(page *Page, keyNum uint16, key uint32) {
	childrenSectionSize := uint32(MaxInternalChildren) * InternalNodeChildSize
	keyOffset := NodeHeaderSize + childrenSectionSize + (uint32(keyNum) * InternalNodeKeySize)
	binary.LittleEndian.PutUint32(page[keyOffset:keyOffset+InternalNodeKeySize], key)
}

func InternalNodeFindChild(page *Page, key uint32) uint32 {
	numKeys := GetNumKeys(page)

	var low uint16 = 0
	var high uint16 = numKeys

	for low < high {
		mid := (low + high) / 2
		k := InternalNodeKey(page, mid)

		if key < k {
			high = mid
		} else {
			low = mid + 1
		}
	}

	return InternalNodeChild(page, low)
}

func CreateRootNode(pager *Pager, rootPageID uint32, leftChildID uint32, rightChildID uint32, splitKey uint32) error {
	rootPage, err := pager.ReadPage(rootPageID)
	if err != nil {
		return err
	}

	SetNodeType(rootPage, NodeTypeInternal)
	SetNumKeys(rootPage, 1)

	SetInternalNodeChild(rootPage, 0, leftChildID)
	SetInternalNodeChild(rootPage, 1, rightChildID)
	SetInternalNodeKey(rootPage, 0, splitKey)

	return pager.WritePage(rootPageID, rootPage)
}

func BTreeSearch(pager *Pager, pageID uint32, key uint32) (*Row, error) {
	page, err := pager.ReadPage(pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to read page %d: %w", pageID, err)
	}

	nodeType := GetNodeType(page)

	// Base Case: Leaf Node
	if nodeType == NodeTypeLeaf {
		slot, found := LeafNodeSearch(page, key)
		if !found {
			return nil, fmt.Errorf("key %d not found", key)
		}
		offset := NodeHeaderSize + (uint32(slot) * RowSize)
		return Deserialize(page[offset : offset+RowSize])
	}

	// Recursive Case: Internal Node
	if nodeType == NodeTypeInternal {
		childPageID := InternalNodeFindChild(page, key)

		if childPageID == pageID {
			return nil, fmt.Errorf("invalid routing: child page ID matches parent page ID %d", pageID)
		}

		return BTreeSearch(pager, childPageID, key)
	}

	return nil, fmt.Errorf("unknown node type %d on page %d", nodeType, pageID)
}

// parent page ID access
func GetParentPageID(page *Page) uint32 {
	return binary.LittleEndian.Uint32(page[HeaderParentOffset : HeaderParentOffset+4])
}

func SetParentPageID(page *Page, parentID uint32) {
	binary.LittleEndian.PutUint32(page[HeaderParentOffset:HeaderParentOffset+4], parentID)
}

func InsertIntoParent(pager *Pager, leftID uint32, rightID uint32, key uint32) error {
	leftPage, err := pager.ReadPage(leftID)
	if err != nil {
		return err
	}

	parentID := GetParentPageID(leftPage)

	// Case 1: leftPage has no parent (splitting the root node)
	if parentID == 0 && leftID == 0 {
		newRootID := uint32(pager.fileSize / PageSize)

		if err := CreateRootNode(pager, newRootID, leftID, rightID, key); err != nil {
			return err
		}

		rightPage, err := pager.ReadPage(rightID)
		if err != nil {
			return err
		}

		SetParentPageID(leftPage, newRootID)
		SetParentPageID(rightPage, newRootID)

		if err := pager.WritePage(leftID, leftPage); err != nil {
			return err
		}
		return pager.WritePage(rightID, rightPage)
	}

	// Case 2: parent already exists -> insert key and right child into parent
	parentPage, err := pager.ReadPage(parentID)
	if err != nil {
		return err
	}

	numKeys := GetNumKeys(parentPage)
	SetInternalNodeChild(parentPage, numKeys+1, rightID)
	SetInternalNodeKey(parentPage, numKeys, key)
	SetNumKeys(parentPage, numKeys+1)

	rightPage, err := pager.ReadPage(rightID)
	if err != nil {
		return err
	}

	SetParentPageID(rightPage, parentID)
	if err := pager.WritePage(rightID, rightPage); err != nil {
		return err
	}

	return pager.WritePage(parentID, parentPage)
}
