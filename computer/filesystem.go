package computer

import (
	"encoding/binary"
	"errors"
	"io"
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

const LATEST_VERSION = 1

const (
	INODESIZE     = 128  // in bytes
	DATABLOCKSIZE = 4096 // in bytes
)

type inode struct {
	size  uint32 // file-size in bytes
	fType InodeType
	refs  uint16

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

	createdAt  uint64
	modifiedAt uint64
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
	disk    *os.File
	suprBlk SuperBlock

	// maybe later cache the bitmaps for extra SPEED.
}

type SuperBlock struct {
	magic   [8]byte // 8 // FS-BS
	version uint32  // 4

	blockSize uint32 // 4

	inodeCount uint32 // 4
	inodeSize  uint32 // 128 // 4

	inodeTableStartBlock  uint32 // 4
	inodeBitmapStartBlock uint32 // 4

	dataBlockCount uint32 // 4

	dataBlockStartBlock  uint32 // 4
	dataBitmapStartBlock uint32 // 4

	totalBlocks uint32 // 4

	// later maybe a dirty bit

	// total: 48
}

// bitmap structure: [inodeCount]uint64

// disk structure

// SUPER BLOCK
// INODE BITMAP
// INODE TABLE
// DATA BITMAP
// DATA BLOCKS

func NewFileSystem(basePath string) *FileSystem {
	// create all directories in basepath
	var soopaStruct SuperBlock

	os.MkdirAll(basePath, 0o755)

	diskPath := filepath.Join(basePath, "disk.img")

	isInitialized := true
	if _, err := os.Stat(diskPath); errors.Is(err, os.ErrNotExist) {
		isInitialized = false
	}

	disk, err := os.OpenFile(
		diskPath,
		os.O_CREATE|os.O_RDWR,
		0o644,
	)
	if err != nil {
		panic(err)
	}

	disk.Truncate((8192 * INODESIZE) + (16384 * DATABLOCKSIZE) + 4096) // double check this sizing TODO add bitmaps

	// now check for the superblk header being correct and up to date.

	disk.Seek(0, io.SeekStart)

	headaBuf := make([]byte, 4096)
	var hedaSupaBlOK SuperBlock
	_, err = io.ReadFull(disk, headaBuf)
	if err != nil {
		panic(err) // maybe dont panic TODO
	}

	copy(hedaSupaBlOK.magic[:], headaBuf[0:8])
	hedaSupaBlOK.version = binary.LittleEndian.Uint32(headaBuf[8:12])
	hedaSupaBlOK.blockSize = binary.LittleEndian.Uint32(headaBuf[12:16])
	hedaSupaBlOK.inodeCount = binary.LittleEndian.Uint32(headaBuf[16:20])
	hedaSupaBlOK.inodeSize = binary.LittleEndian.Uint32(headaBuf[20:24])
	hedaSupaBlOK.inodeTableStartBlock = binary.LittleEndian.Uint32(headaBuf[24:28])
	hedaSupaBlOK.inodeBitmapStartBlock = binary.LittleEndian.Uint32(headaBuf[28:32])
	hedaSupaBlOK.dataBlockCount = binary.LittleEndian.Uint32(headaBuf[32:36])
	hedaSupaBlOK.dataBlockStartBlock = binary.LittleEndian.Uint32(headaBuf[36:40])
	hedaSupaBlOK.dataBitmapStartBlock = binary.LittleEndian.Uint32(headaBuf[40:44])
	hedaSupaBlOK.totalBlocks = binary.LittleEndian.Uint32(headaBuf[44:48])

	if string(hedaSupaBlOK.magic[:5]) != "FS-BS" {
		//log.Println("Invalid magic: expected FS-BS, got %s", hedaSupaBlOK.magic)
		isInitialized = false
	}

	if hedaSupaBlOK.version != LATEST_VERSION {
		//log.Println("FS NOT ON LATEST VERIZON!") // will log later, for no for debugging no need
		isInitialized = false
	}

	if !isInitialized {
		// format the fs

		numInodes := uint32(8192)
		numBlocks := uint32(16384)

		suprBuf := make([]byte, 4096)
		suprBlk := SuperBlock{
			magic:          [8]byte{'F', 'S', '-', 'B', 'S'},
			version:        1,
			blockSize:      4096,
			inodeCount:     numInodes,
			inodeSize:      INODESIZE,
			dataBlockCount: numBlocks,
			// offsets TODO
			inodeBitmapStartBlock: 1,
			inodeTableStartBlock:  2, // (adjust based on bitmap size)
		}

		// Indexing is [inclusive]:[exclusive]
		copy(suprBuf[0:8], suprBlk.magic[:])
		binary.LittleEndian.PutUint32(suprBuf[8:12], suprBlk.version)
		binary.LittleEndian.PutUint32(suprBuf[12:16], suprBlk.blockSize)
		binary.LittleEndian.PutUint32(suprBuf[16:20], suprBlk.inodeCount)
		binary.LittleEndian.PutUint32(suprBuf[20:24], suprBlk.inodeSize)
		binary.LittleEndian.PutUint32(suprBuf[24:28], suprBlk.inodeTableStartBlock)
		binary.LittleEndian.PutUint32(suprBuf[28:32], suprBlk.inodeBitmapStartBlock)
		binary.LittleEndian.PutUint32(suprBuf[32:36], suprBlk.dataBlockCount)
		binary.LittleEndian.PutUint32(suprBuf[36:40], suprBlk.dataBlockStartBlock)
		binary.LittleEndian.PutUint32(suprBuf[40:44], suprBlk.dataBitmapStartBlock)
		binary.LittleEndian.PutUint32(suprBuf[44:48], suprBlk.totalBlocks)

		_, _ = disk.WriteAt(suprBuf, 0) // add error handling TODO
		soopaStruct = suprBlk
	} else {
		soopaStruct = hedaSupaBlOK
	}

	// closing will be done when the engine shuts down. TODO
	return &FileSystem{
		suprBlk: soopaStruct,
	}
}
