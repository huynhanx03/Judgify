---
trigger: always_on
---

Naming: short, meaningful names, avoid unclear abbreviations
File Naming: lowercase, use snake_case, name by responsibility, avoid generic names like helper or common
Folder Naming: lowercase only, no underscores, name by responsibility not by layer, avoid utils and common
Code Style: prefer early return, single responsibility per function, comments explain WHY not WHAT
Performance: reuse buffers, use atomic for simple counters, avoid reflection in hot paths
Memory: pre-allocate slices and maps, use pointer receivers for large structs
Concurrency: always pass context.Context, define goroutine lifecycle clearly, avoid goroutine leaks
Error Handling: wrap errors with context, use sentinel errors for comparison