# Go Custom Database Engine

A low-level, page-based relational database storage engine built from scratch in Go. It handles low-level binary serialization, custom 4KB page management, dynamic B+ Tree indexing, and a thread-safe LRU Buffer Pool with dirty-page write-back tracking.

## ✨ Features

* **Custom 4KB Pager Architecture:** Operates on fixed-size 4096-byte disk pages using direct OS file I/O (`os.File`) to manage binary page offsets.
* **Thread-Safe LRU Buffer Pool:** Sits between the storage engine and disk to cache active pages in RAM using a hash map, doubly linked list, and `sync.Mutex` concurrency controls.
* **Write-Back Caching:** Defers expensive disk I/O by marking modified pages as dirty and flushing them to `db.bin` upon LRU eviction or graceful shutdown (`FlushAll`).
* **Binary Row Serialization:** Encodes structured row data into compact binary payloads using fixed field offsets and Little-Endian byte order.
* **Dynamic B+ Tree Indexing:** Implements self-sorting B+ Tree nodes with $O(\log N)$ binary search lookups, cascading internal node splits, and dynamic root promotions.
* **Sequential Range Scanning:** Connects leaf nodes using embedded header pointers (`NextPageID`), enabling fast $O(K)$ range scans across page boundaries without re-traversing parent nodes.

## 🛠 Tech Stack

* **Language:** Go (Golang)
* **Caching Layer:** Thread-Safe LRU Buffer Pool (`sync.Mutex`, Write-Back Dirty Tracking)
* **Storage Layer:** Custom Binary Storage Engine & File Pager
* **Data Structure:** B+ Tree Indexing (Leaf Nodes & Internal Routing Pages)
* **OS Systems API:** File I/O via `os.File` (`ReadAt`, `WriteAt`, `Sync`)

## 📋 Page Layout & Header Metadata

Every 4KB page reserves an 11-byte metadata header section:

| Offset | Size | Field | Description |
|---|---|---|---|
| `0` | 1 byte | `NodeType` | `1` = Internal Node, `2` = Leaf Node |
| `1` | 2 bytes | `NumKeys` | Total active keys/rows stored in page |
| `3` | 4 bytes | `ParentID` | Page ID of parent internal node |
| `7` | 4 bytes | `NextID` | Page ID of next sibling leaf page (Range Scans) |
