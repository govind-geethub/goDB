# Go Custom Database Engine

A lightweight, high-performance relational database storage engine built from scratch in Go. It handles low-level binary serialization, custom 4KB page management, and persistent on-disk B-Tree indexing to ensure $O(\log N)$ search speed and power-loss durability.

## ✨ Features
* **Custom 4KB Pager Architecture:** Operates on fixed-size 4096-byte disk pages with explicit OS page cache flushing (`fsync`) for ACID durability.
* **Binary Row Serialization:** Efficiently encodes structured row data into compact 291-byte binary payloads using fixed field offsets and Little-Endian encoding.
* **Dynamic B-Tree Indexing:** Implements self-sorting B-Tree leaf nodes with binary search lookup ($O(\log N)$) and automatic 50/50 node splitting when pages exceed capacity.
* **Sequential Range Scanning:** Connects leaf nodes using embedded header pointers (`NextPageID`), enabling fast range scans across page boundaries.

## 🛠️ Tech Stack
* **Language:** Go (Golang)
* **Storage Layer:** Custom Binary Storage Engine & File Pager
* **Data Structure:** B-Tree Indexing (Leaf Nodes & Internal Pages)
* **OS Systems API:** Direct file I/O via `os.File` (`ReadAt`, `WriteAt`, `Sync`)
