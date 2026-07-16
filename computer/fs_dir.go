package computer

// DIRECTORY ENTRIES
// - 64-byte fixed-size entries, format in FILESYSTEM.md "Directory Entry Format"
// - encodeDirEntry(name, inum) [64]byte  — free function, pure byte math, no disk
// - decodeDirEntries(block) []DirEntry   — walks 64 entries, skips inum==0
// - DirectoryOps struct (holds inodeNum) + ReadEntries + CreateFD
//
// The encode/decode functions are free functions (no *FileSystem receiver), matching
// writeSuprBlktoSuprBuf / setBit / findFreeBit — pure byte helpers that don't touch
// disk directly. DirectoryOps methods take *Kernel to reach the fs.
