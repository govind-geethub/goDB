package main

import (
	"fmt"
	"os"
)

const PageSize = 4096 // bytes

// Page is representing 1 page of 4KB size (const array)
type Page [PageSize]byte

// reading and writing pages to binary in disk
type Pager struct {
	file     *os.File
	fileSize int64
}

// func opens a existing database file or create new one
func NewPager(filename string) (*Pager, error) {

	// open file with r/w permission, create if missing, set file permission to 0666
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("Failed to open database file: %w", err)
	}

	// file metadata to get its curr size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("Failed to get file info: %w", err)
	}

	return &Pager{
		file:     file,
		fileSize: info.Size(),
	}, nil
}

// ReadPage feteches specific 4KB page from the disk by PageID
func (p *Pager) ReadPage(pageID uint32) (*Page, error) {
	var page Page
	offset := int64(pageID) * PageSize

	// if req page is out of bounds return blank
	if offset >= p.fileSize {
		return &page, nil
	}

	// reading 4KB from the offset
	_, err := p.file.ReadAt(page[:], offset)
	if err != nil {
		return nil, fmt.Errorf("Failed to read page %d from disk: %w", pageID, err)
	}

	return &page, nil
}
