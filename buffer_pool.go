package main

import (
	"container/list"
	"fmt"
	"sync"
)

type CacheFrame struct {
	pageID  uint32
	page    *Page
	isDirty bool
}

type bufferPool struct {
	capacity int
	pager    *Pager
	cache    map[uint32]*list.Element
	lruList  *list.List
	mu       sync.Mutex
}

func NewBufferPool(capacity int, pager *Pager) *bufferPool {
	return &bufferPool{
		capacity: capacity,
		pager:    pager,
		cache:    make(map[uint32]*list.Element),
		lruList:  list.New(),
	}
}

// retrieves page from memory, or if missed then from disk
func (bp *bufferPool) FetchPage(pageID uint32) (*Page, error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// cache hit
	if elem, exists := bp.cache[pageID]; exists {
		bp.lruList.MoveToFront(elem)
		return elem.Value.(*CacheFrame).page, nil
	}

	// cache miss
	page, err := bp.pager.ReadPage(pageID)
	if err != nil {
		return nil, err
	}

	// evict if pool capacity reached
	if bp.lruList.Len() >= bp.capacity {
		if err := bp.evictLRU(); err != nil {
			return nil, fmt.Errorf("Buffer pool eviction failed: %w", err)
		}
	}

	// add new frame to cache
	frame := &CacheFrame{
		pageID:  pageID,
		page:    page,
		isDirty: false,
	}

	elem := bp.lruList.PushFront(frame)
	bp.cache[pageID] = elem

	return page, nil
}

// marks a page in cache as modified so it flushes to disk on eviction
func (bp *bufferPool) MarkDIrty(pageID uint32) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if elem, exists := bp.cache[pageID]; exists {
		elem.Value.(*CacheFrame).isDirty = true
		bp.lruList.MoveToFront(elem)
	}
}

// flushes dirty page if modified, then removes oldest frame from cache
func (bp *bufferPool) evictLRU() error {
	backElem := bp.lruList.Back()
	if backElem == nil {
		return nil
	}

	frame := backElem.Value.(*CacheFrame)
	if frame.isDirty {
		if err := bp.pager.WritePage(frame.pageID, frame.page); err != nil {
			return err
		}
	}

	delete(bp.cache, frame.pageID)
	bp.lruList.Remove(backElem)
	return nil
}

// writes all modified dirty pages in the pool to disk
func (bp *bufferPool) FlushAll() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for _, elem := range bp.cache {
		frame := elem.Value.(*CacheFrame)
		if frame.isDirty {
			if err := bp.pager.WritePage(frame.pageID, frame.page); err != nil {
				return err
			}
			frame.isDirty = false
		}
	}
	return nil
}
