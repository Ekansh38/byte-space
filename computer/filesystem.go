package computer

// PLANNING
// inode file: meta data (file-size, type) + block list + FIND, SIND, TIND
// data file: raw bytes (fixed size)
// directory data block entry (fixed size)
// if string too short ["p", "i", "c", "s", "\0","0","0","0"] // the extra 0's are padding
// metadata: freeBlocks + root
// write: take free block -> store data -> update inode
// read: inode -> blocks -> combine
// delete: free blocks -> remove inode
// freeBlocks empty = disk full (FAHH)

type inode struct {
	size uint32  // file-size in bytes
	meta FileMetadata
	fType FDType
}

type FileMetadata struct {
	Filepath string // the file/folder this metadata applies to

	Owner string // the owner of the file/folder,
	// string of the username of the creator,
	// for system files it is root, for other stuff /home/user it is that user.

	Setuid bool // true means the user who runs that program can run it in the permissions of the owner

	OwnerMode uint8
	OtherMode uint8

	//    rwx     // read  write  execute permissions
	// 0: 000
	// 1: 001
	// 2: 010
	// 3: 011
	// 4: 100
	// 5: 101
	// 6: 110
	// 7: 111
}
