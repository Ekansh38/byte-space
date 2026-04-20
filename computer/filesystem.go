package computer

import (
	"errors"
	"os"
	"path/filepath"
)

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

type InodeType uint8

const (
	S_IFREG = 0
	S_IFDIR = 1
)

const (
	INODESIZE     = 86   // in bytes
	DATABLOCKSIZE = 4096 // in bytes
)

type inode struct {
	size  uint32 // file-size in bytes
	fType InodeType

	owner [14]byte // the owner of the file/folder,
	// string of the username of the creator,
	// for system files it is root, for other stuff /home/user it is that user.

	setuid bool // true means the user who runs that program can run it in the permissions of the owner

	ownerMode uint8
	otherMode uint8

	//    rwx     // read  write  execute permissions
	// 0: 000
	// 1: 001
	// 2: 010
	// 3: 011
	// 4: 100
	// 5: 101
	// 6: 110
	// 7: 111

	// BLOCK LISTS

	direct [12]uint32
	find   uint32 // first-indirect
	sind   uint32 // second-indirect
	tind   uint32 // third-indirect
}

type dataBlockType int

const (
	REG      = 0 // not a directory data block
	DIRBLOCK = 1
)

type dataBlock struct {
	blockType dataBlockType
	data      [DATABLOCKSIZE]byte // key-value pair type data block if it's a directory data block.
}

type FileSystem struct {
	inodeFile *os.File
	dataFile  *os.File
	metaFile  *os.File // will store freeblocks and freeinodes bitmaps

	inodeCount int
	blockCount int
}

func NewFileSystem(basePath string) *FileSystem {
	isInitialized := true

	inodeFilePath := filepath.Join(basePath, "inode_table.bin")
	dataFilePath := filepath.Join(basePath, "data_blocks.bin")
	metaFilePath := filepath.Join(basePath, "disk_meta.bin")

	if _, err := os.Stat(inodeFilePath); errors.Is(err, os.ErrNotExist) {
		isInitialized = false
	} else if _, err := os.Stat(dataFilePath); errors.Is(err, os.ErrNotExist) {
		isInitialized = false
	} else if _, err := os.Stat(metaFilePath); errors.Is(err, os.ErrNotExist) {
		isInitialized = false
	}

	inodeFile, err := os.OpenFile(
		inodeFilePath,
		os.O_CREATE|os.O_RDWR,
		0o644,
	)
	if err != nil {
		panic(err)
	}

	dataFile, err := os.OpenFile(
		dataFilePath,
		os.O_CREATE|os.O_RDWR,
		0o644,
	)
	if err != nil {
		panic(err)
	}

	metaFile, err := os.OpenFile(
		metaFilePath,
		os.O_CREATE|os.O_RDWR,
		0o644,
	)
	if err != nil {
		panic(err)
	}

	inodeFile.Truncate(8192 * INODESIZE)
	dataFile.Truncate(16384 * DATABLOCKSIZE)
	metaFile.Truncate(4096)


	if !isInitialized {

		// do formatting work.
	}

	defer metaFile.Close()
	defer dataFile.Close()
	defer inodeFile.Close()


	return &FileSystem{}
}
