package main

import "encoding/binary"

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
