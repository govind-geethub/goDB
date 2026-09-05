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

func GetParentPageID(page *Page) uint32 {
	return binary.LittleEndian.Uint32(page[HeaderParentOffset : HeaderParentOffset+4])
}

func SetParentPageID(page *Page, parentID uint32) {
	binary.LittleEndian.PutUint32(page[HeaderParentOffset:HeaderParentOffset+4], parentID)
}

// GetRootPageID walks parent pointers up from Page 0 to locate top root node ID
func GetRootPageID(pool *BufferPool) (uint32, error) {
	p0, err := pool.FetchPage(0)
	if err != nil {
		return 0, err
	}

	parentID := GetParentPageID(p0)
	if parentID == 0 {
		return 0, nil
	}

	currID := parentID
	for {
		parentPage, err := pool.FetchPage(currID)
		if err != nil {
			return currID, nil
		}
		nextParent := GetParentPageID(parentPage)
		if nextParent == 0 {
			break
		}
		currID = nextParent
	}

	return currID, nil
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
	maxKeys := uint16((PageSize - NodeHeaderSize) / RowSize)

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

func LeafNodeInsertOrSplit(pool *BufferPool, pageID uint32, page *Page, row *Row) error {
	numKeys := GetNumKeys(page)
	maxKeys := uint16((PageSize - NodeHeaderSize) / RowSize)

	if numKeys >= maxKeys {
		_, err := LeafNodeSplit(pool, pageID, page, row)
		if err != nil {
			return err
		}

		updatedPage, err := pool.FetchPage(pageID)
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
	pool.MarkDirty(pageID)
	return nil
}

func LeafNodeSplit(pool *BufferPool, oldPageID uint32, oldPage *Page, newRow *Row) (uint32, error) {
	numKeys := GetNumKeys(oldPage)

	rows := make([]Row, 0, numKeys+1)
	for i := uint16(0); i < numKeys; i++ {
		offset := NodeHeaderSize + (uint32(i) * RowSize)
		r, err := Deserialize(oldPage[offset : offset+RowSize])
		if err != nil {
			return 0, fmt.Errorf("failed to deserialize row during splitting: %w", err)
		}
		rows = append(rows, *r)
	}

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

	newPageID := uint32(pool.pager.fileSize / PageSize)
	newPage, err := pool.FetchPage(newPageID)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate new page for split: %w", err)
	}

	SetNodeType(newPage, NodeTypeLeaf)
	SetNumKeys(newPage, 0)
	SetNextPage(newPage, GetNextPage(oldPage))

	SetNextPage(oldPage, newPageID)

	for i := NodeHeaderSize; i < PageSize; i++ {
		oldPage[i] = 0
	}
	SetNodeType(oldPage, NodeTypeLeaf)
	SetNumKeys(oldPage, 0)

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

	pool.MarkDirty(oldPageID)
	pool.MarkDirty(newPageID)

	splitKey := rows[mid].ID
	err = InsertIntoParent(pool, oldPageID, newPageID, splitKey)
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

		if key >= k {
			low = mid + 1
		} else {
			high = mid
		}
	}

	return InternalNodeChild(page, low)
}

func CreateRootNode(pool *BufferPool, rootPageID uint32, leftChildID uint32, rightChildID uint32, splitKey uint32) error {
	rootPage, err := pool.FetchPage(rootPageID)
	if err != nil {
		return err
	}

	SetNodeType(rootPage, NodeTypeInternal)
	SetNumKeys(rootPage, 1)

	SetInternalNodeChild(rootPage, 0, leftChildID)
	SetInternalNodeChild(rootPage, 1, rightChildID)
	SetInternalNodeKey(rootPage, 0, splitKey)

	pool.MarkDirty(rootPageID)
	return nil
}

func BTreeSearch(pool *BufferPool, pageID uint32, key uint32) (*Row, error) {
	page, err := pool.FetchPage(pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page %d: %w", pageID, err)
	}

	nodeType := GetNodeType(page)

	if nodeType == NodeTypeLeaf {
		slot, found := LeafNodeSearch(page, key)
		if !found {
			return nil, fmt.Errorf("key %d not found", key)
		}
		offset := NodeHeaderSize + (uint32(slot) * RowSize)
		return Deserialize(page[offset : offset+RowSize])
	}

	if nodeType == NodeTypeInternal {
		childPageID := InternalNodeFindChild(page, key)
		if childPageID == pageID {
			return nil, fmt.Errorf("invalid routing loop on page %d", pageID)
		}
		return BTreeSearch(pool, childPageID, key)
	}

	return nil, fmt.Errorf("unknown node type %d on page %d", nodeType, pageID)
}

func InsertIntoParent(pool *BufferPool, leftID uint32, rightID uint32, key uint32) error {
	leftPage, err := pool.FetchPage(leftID)
	if err != nil {
		return err
	}

	parentID := GetParentPageID(leftPage)

	if parentID == 0 {
		newRootID := uint32(pool.pager.fileSize / PageSize)

		if err := CreateRootNode(pool, newRootID, leftID, rightID, key); err != nil {
			return err
		}

		rightPage, err := pool.FetchPage(rightID)
		if err != nil {
			return err
		}

		SetParentPageID(leftPage, newRootID)
		SetParentPageID(rightPage, newRootID)

		pool.MarkDirty(leftID)
		pool.MarkDirty(rightID)
		return nil
	}

	parentPage, err := pool.FetchPage(parentID)
	if err != nil {
		return err
	}

	numKeys := GetNumKeys(parentPage)

	if numKeys >= uint16(MaxInternalChildren-1) {
		_, err := InternalNodeSplit(pool, parentID, parentPage, rightID, key)
		return err
	}

	keys := make([]uint32, 0, numKeys+1)
	children := make([]uint32, 0, numKeys+2)

	for i := uint16(0); i <= numKeys; i++ {
		children = append(children, InternalNodeChild(parentPage, i))
	}
	for i := uint16(0); i < numKeys; i++ {
		keys = append(keys, InternalNodeKey(parentPage, i))
	}

	inserted := false
	for i, k := range keys {
		if key < k {
			keys = append(keys[:i], append([]uint32{key}, keys[i:]...)...)
			children = append(children[:i+1], append([]uint32{rightID}, children[i+1:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		keys = append(keys, key)
		children = append(children, rightID)
	}

	SetNumKeys(parentPage, uint16(len(keys)))
	for i := 0; i < len(keys); i++ {
		SetInternalNodeKey(parentPage, uint16(i), keys[i])
	}
	for i := 0; i < len(children); i++ {
		SetInternalNodeChild(parentPage, uint16(i), children[i])
	}

	rightPage, err := pool.FetchPage(rightID)
	if err != nil {
		return err
	}

	SetParentPageID(rightPage, parentID)
	pool.MarkDirty(rightID)
	pool.MarkDirty(parentID)

	return nil
}

func InternalNodeSplit(pool *BufferPool, oldPageID uint32, oldPage *Page, newChildID uint32, newKey uint32) (uint32, error) {
	numKeys := GetNumKeys(oldPage)

	keys := make([]uint32, 0, numKeys+1)
	children := make([]uint32, 0, numKeys+2)

	for i := uint16(0); i <= numKeys; i++ {
		children = append(children, InternalNodeChild(oldPage, i))
	}
	for i := uint16(0); i < numKeys; i++ {
		keys = append(keys, InternalNodeKey(oldPage, i))
	}

	inserted := false
	for i, k := range keys {
		if newKey < k {
			keys = append(keys[:i], append([]uint32{newKey}, keys[i:]...)...)
			children = append(children[:i+1], append([]uint32{newChildID}, children[i+1:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		keys = append(keys, newKey)
		children = append(children, newChildID)
	}

	newPageID := uint32(pool.pager.fileSize / PageSize)
	newPage, err := pool.FetchPage(newPageID)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate new internal page: %w", err)
	}

	SetNodeType(newPage, NodeTypeInternal)
	SetParentPageID(newPage, GetParentPageID(oldPage))

	midKeyIndex := len(keys) / 2
	promotedKey := keys[midKeyIndex]

	for i := NodeHeaderSize; i < PageSize; i++ {
		oldPage[i] = 0
	}
	SetNodeType(oldPage, NodeTypeInternal)

	for i := 0; i < midKeyIndex; i++ {
		SetInternalNodeKey(oldPage, uint16(i), keys[i])
	}
	for i := 0; i <= midKeyIndex; i++ {
		SetInternalNodeChild(oldPage, uint16(i), children[i])
	}
	SetNumKeys(oldPage, uint16(midKeyIndex))

	rightKeyCount := len(keys) - midKeyIndex - 1
	SetNumKeys(newPage, uint16(rightKeyCount))

	for i := 0; i < rightKeyCount; i++ {
		SetInternalNodeKey(newPage, uint16(i), keys[midKeyIndex+1+i])
	}
	for i := 0; i <= rightKeyCount; i++ {
		childID := children[midKeyIndex+1+i]
		SetInternalNodeChild(newPage, uint16(i), childID)

		childPage, err := pool.FetchPage(childID)
		if err == nil {
			SetParentPageID(childPage, newPageID)
			pool.MarkDirty(childID)
		}
	}

	pool.MarkDirty(oldPageID)
	pool.MarkDirty(newPageID)

	err = InsertIntoParent(pool, oldPageID, newPageID, promotedKey)
	if err != nil {
		return 0, fmt.Errorf("failed cascading internal split: %w", err)
	}

	return newPageID, nil
}

func FindLeafPage(pool *BufferPool, pageID uint32, key uint32) (uint32, error) {
	page, err := pool.FetchPage(pageID)
	if err != nil {
		return 0, err
	}

	if GetNodeType(page) == NodeTypeLeaf {
		return pageID, nil
	}

	childID := InternalNodeFindChild(page, key)
	return FindLeafPage(pool, childID, key)
}

func BTreeScanRange(pool *BufferPool, rootID uint32, startKey uint32, endKey uint32) ([]Row, error) {
	if startKey > endKey {
		return nil, fmt.Errorf("Invalid Range start %d > end %d", startKey, endKey)
	}

	currPageID, err := FindLeafPage(pool, rootID, startKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to find the start leaf page: %w", err)
	}

	var results []Row

	for {
		page, err := pool.FetchPage(currPageID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch leaf page %d: %w", currPageID, err)
		}

		numKeys := GetNumKeys(page)
		for i := uint16(0); i < numKeys; i++ {
			offset := NodeHeaderSize + (uint32(i) * RowSize)
			rowID := binary.LittleEndian.Uint32(page[offset : offset+4])

			if rowID >= startKey && rowID <= endKey {
				row, err := Deserialize(page[offset : offset+RowSize])
				if err != nil {
					return nil, fmt.Errorf("Deserialization error: %w", err)
				}
				results = append(results, *row)
			}

			if rowID > endKey {
				return results, nil
			}
		}

		nextPage := GetNextPage(page)
		if nextPage == 0 {
			break
		}
		currPageID = nextPage
	}

	return results, nil
}
