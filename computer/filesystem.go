package computer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// PLANNING
// if string too short ["p", "i", "c", "s", "\0","0","0","0"] // the extra 0's are padding

type InodeType uint8

const (
	S_IFREG = 0
	S_IFDIR = 1
)

type InodeOperations interface {
	CreateFD()
}

const LATEST_VERSION = 10

const (
	INODESIZE   = 0x80   // 128 in bytes
	BLOCKSIZE   = 0x1000 // 4096 in bytes
	DISKSIZE    = 67_108_864
	INODES      = 8064
	BLOCKS      = 8064 * 2
	MAGICLEN    = 8
	TOTALBLOCKS = 16384
)

type inode struct {
	size  uint32 // file-size in bytes
	fType InodeType
	refs  uint16

	owner [14]byte // the owner of the file/folder, // TODO cap the username len to 13 in the kernel
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

	direct [12]uint32 // 12 x4 = 48
	find   uint32     // first-indirect
	sind   uint32     // second-indirect
	tind   uint32     // third-indirect

	// sind and tind are not unix style recurisve indirection just 1024 + 1024 + 1024. simple

	createdAt  uint64
	modifiedAt uint64

	// runtime only
	num int
	ops InodeOperations
}

type dataBlockType int

const (
	REG      = 0 // not a directory data block
	DIRBLOCK = 1
)

type dataBlock struct {
	blockType dataBlockType
	data      [BLOCKSIZE]byte // key-value pair type data block if it's a directory data block.
}

type FileSystem struct {
	disk     *os.File
	superBlk SuperBlock // cached

	// maybe later cache the bitmaps for extra SPEED.
}

type SuperBlock struct {
	magic   [8]byte // BS-EXTFS
	version uint32  // 1

	blockSize uint32 // 4096

	inodeCount uint32 // ~8192
	inodeSize  uint32 // 128

	inodeBitmapStartBlock uint32 // 1
	inodeTableStartBlock  uint32 // 2

	dataBlockCount uint32 // 16384

	dataBlocksStartBlock uint32 // 255
	dataBitmapStartBlock uint32 // 254

	totalBlocks uint32 // 16384

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
	var cachedSuperBlock SuperBlock

	os.MkdirAll(basePath, 0o755)
	diskPath := filepath.Join(basePath, "disk.img")

	// Check formatting
	isInitialized := true
	if _, err := os.Stat(diskPath); errors.Is(err, os.ErrNotExist) {
		log.Println("Disk not created yet.")
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

	disk.Truncate(DISKSIZE)

	// now check for the superblk header being correct and up to date.

	disk.Seek(0, io.SeekStart)

	headerBuf := make([]byte, BLOCKSIZE)
	var headerSuperBlk SuperBlock
	_, err = io.ReadFull(disk, headerBuf)
	if err != nil {
		panic(err)
	}

	// Only get the header data to validate.

	copy(headerSuperBlk.magic[:], headerBuf[0:MAGICLEN])
	headerSuperBlk.version = binary.LittleEndian.Uint32(headerBuf[8:12])

	// check if the header is valid
	if string(headerSuperBlk.magic[:MAGICLEN]) != "BS-EXTFS" && isInitialized == true {
		log.Println("Invalid magic: expected BS-EXTFS, got %s", headerSuperBlk.magic)
		isInitialized = false
	}
	if headerSuperBlk.version != LATEST_VERSION && isInitialized == true {
		log.Println("Invalid version: expected %d, got %d", LATEST_VERSION, headerSuperBlk.version)
		isInitialized = false
	}

	if !isInitialized {
		// format the fs

		// If its not valid we need to create and initialize a new disk.img

		superBuf := make([]byte, BLOCKSIZE)
		superBlk := SuperBlock{
			magic:   [MAGICLEN]byte{'B', 'S', '-', 'E', 'X', 'T', 'F', 'S'},
			version: LATEST_VERSION,

			blockSize: BLOCKSIZE,
			inodeSize: INODESIZE,

			inodeCount:     uint32(INODES),
			dataBlockCount: uint32(BLOCKS),

			inodeBitmapStartBlock: 1,
			inodeTableStartBlock:  2,

			dataBitmapStartBlock: 254,
			dataBlocksStartBlock: 255,

			totalBlocks: TOTALBLOCKS,
		}

		writeSuprBlktoSuprBuf(superBuf, superBlk)

		_, _ = disk.WriteAt(superBuf, 0)

		cachedSuperBlock = superBlk

		// next we need to format the inode bitmap and maybe cache it.

		inodeBitmapBuf := make([]byte, BLOCKSIZE)

		inodeBitmapBuf[0] = 0b00000111 // the 0 1 2 inodes are taken, 2 is root.

		// so the bytes are left to right, but in a byte its right to left. odd ik but yea.

		// write to disk

		_, _ = disk.WriteAt(inodeBitmapBuf, BLOCKSIZE*int64(superBlk.inodeBitmapStartBlock))

		// inodes into disk

		inodeTableBuf := make([]byte, BLOCKSIZE*252)
		// inodeTableBuf[0] = 0b11111111
		// inodeTableBuf[252*DATABLOCKSIZE-1] = 0b11111111

		_, _ = disk.WriteAt(inodeTableBuf, BLOCKSIZE*int64(superBlk.inodeTableStartBlock))

		// data bitmap

		dataBitmapBuf := make([]byte, BLOCKSIZE)
		// dataBitmapBuf[0] = 0b10101010
		dataBitmapBuf[BLOCKSIZE-1] = 0b10101010
		disk.WriteAt(dataBitmapBuf, int64(superBlk.dataBitmapStartBlock)*BLOCKSIZE)

		// data blocks
		dataBlocksBuf := make([]byte, BLOCKSIZE*superBlk.dataBlockCount)
		// dataBlocksBuf[0] = 0b10101010
		// dataBlocksBuf[(DATABLOCKSIZE*superBlk.dataBlockCount)-1] = 0b10101010
		disk.WriteAt(dataBlocksBuf, int64(superBlk.dataBlocksStartBlock)*BLOCKSIZE)

		// padding

		paddingBuf := make([]byte, BLOCKSIZE)
		for idx := range paddingBuf {
			paddingBuf[idx] = 67
		}
		disk.WriteAt(paddingBuf, int64(superBlk.dataBlocksStartBlock+BLOCKS)*BLOCKSIZE)

	} else {
		// Copy from buffer into go struct data structure

		headerSuperBlk.blockSize = binary.LittleEndian.Uint32(headerBuf[12:16])
		headerSuperBlk.inodeCount = binary.LittleEndian.Uint32(headerBuf[16:20])
		headerSuperBlk.inodeSize = binary.LittleEndian.Uint32(headerBuf[20:24])
		headerSuperBlk.inodeTableStartBlock = binary.LittleEndian.Uint32(headerBuf[24:28])
		headerSuperBlk.inodeBitmapStartBlock = binary.LittleEndian.Uint32(headerBuf[28:32])
		headerSuperBlk.dataBlockCount = binary.LittleEndian.Uint32(headerBuf[32:36])
		headerSuperBlk.dataBlocksStartBlock = binary.LittleEndian.Uint32(headerBuf[36:40])
		headerSuperBlk.dataBitmapStartBlock = binary.LittleEndian.Uint32(headerBuf[40:44])
		headerSuperBlk.totalBlocks = binary.LittleEndian.Uint32(headerBuf[44:48])

		cachedSuperBlock = headerSuperBlk
	}

	fs := &FileSystem{
		superBlk: cachedSuperBlock,
		disk:     disk,
	}

	if !isInitialized {
		// create the root inode

		// passing a pointer cuz less memory, ik it doesnt need to mutate.
		fs.writeInode(&inode{
			size:  0,
			fType: S_IFDIR,
			refs:  0,
			owner: [14]byte{'r', 'o', 'o', 't'},

			setuid:     false,
			ownerMode:  0b111, // 7
			otherMode:  0b101, // 5
			direct:     [12]uint32{},
			find:       0,
			sind:       0,
			tind:       0,
			createdAt:  uint64(time.Now().Unix()),
			modifiedAt: uint64(time.Now().Unix()),
		}, 2)
	}

	return fs

	// falloc and write to inode
}

func (fs *FileSystem) falloc(inode *inode, newSize uint32) error {
	// keep in mind this function does assume everything is perfect and correct about the inode.
	// its a very low level function, it does not perform any checks.
	// that is for higher level kernel/filesystem commands to enforce and perform on programs making syscalls.

	blocksNeeded := (newSize + BLOCKSIZE - 1) / BLOCKSIZE
	// same: ceil(float(newSize) / float(BLOCKSIZE))

	numOCurrentBlocks := (inode.size + BLOCKSIZE - 1) / BLOCKSIZE
	// same: ceil(float(inode.size) / float(BLOCKSIZE))

	// if they have enough blocks, even if the size is higher. Eg. size = 10, falloc(20). WE DONT GOTTA DO ANY WORK!!
	// they already have a 4096 block.

	if blocksNeeded > numOCurrentBlocks {
		// alloc more blocks.
	} else if blocksNeeded < numOCurrentBlocks {
		// shrink

		for i := blocksNeeded; i < numOCurrentBlocks; i++ {
			// i = just the virtual block number
			// we need to free all these blocks.

			// convert i to either. direct[x], find[x], sind[x], tind[x]. get that uint32, free it. and 0 the value.

			physicalAddress := i
			place := 0 // 0 = direct, 1 = find, 2 = sind, 3 = tind

			if i > 11 && i < 1036{
				physicalAddress = i-12
				place = 1
			} else if i >= 1036 && i < 2_060{
				physicalAddress = i-1036
				place = 2
			} else if i >= 2_060 {
				physicalAddress = i-2_060
				place = 3
			}

			if place == 0 {
				inode.direct[physicalAddress] = 0
				// free that too

			} else if place == 1 {
				// put it in there
				// Get the block address at physicalAddress index
				// free blockAddr
				// Zero it out
				// Write back

			} else if place == 2 {
				// Get the second level indirect block address
				// Get the actual data block address
				// free blockAddr
				// Zero it out
				// Write back

			} else if place == 3 {
				// Get second level indirect block address
				// Get third level indirect block address
				// Get the actual data block address
				// free blockAddr
				// Zero it out
				// Write back

			}

		}


	}

	return nil
}

func (fs *FileSystem) readInode(inode *inode, idx int) error {
	// offset
	inode.num = idx
	inodeTableOffset := int(fs.superBlk.inodeTableStartBlock) * BLOCKSIZE
	startIdx := inodeTableOffset + idx*INODESIZE
	inodeBuf := make([]byte, INODESIZE) // 128

	// Read from disk
	n, err := fs.disk.ReadAt(inodeBuf, int64(startIdx))
	if err != nil {
		return err
	}
	if n != INODESIZE {
		return io.ErrUnexpectedEOF
	}

	// Deserialize from buffer into inode struct
	inode.size = binary.LittleEndian.Uint32(inodeBuf[0:4])
	inode.fType = InodeType(inodeBuf[4])
	inode.refs = binary.LittleEndian.Uint16(inodeBuf[5:7])

	//if inode.fType == S_IFREG {
	//	inode.ops = RegOps
	//}
	// TODO

	for idx, val := range inodeBuf[7:21] {
		inode.owner[idx] = val
	}

	if inodeBuf[21] == 1 {
		inode.setuid = true
	} else {
		inode.setuid = false
	}

	inode.ownerMode = inodeBuf[22]
	inode.otherMode = inodeBuf[23]

	for idx := range inode.direct {
		off := idx * 4
		inode.direct[idx] = binary.LittleEndian.Uint32(inodeBuf[24+off : 28+off])
	}

	inode.find = binary.LittleEndian.Uint32(inodeBuf[72:76])
	inode.sind = binary.LittleEndian.Uint32(inodeBuf[76:80])
	inode.tind = binary.LittleEndian.Uint32(inodeBuf[80:84])

	inode.createdAt = binary.LittleEndian.Uint64(inodeBuf[84:92])
	inode.modifiedAt = binary.LittleEndian.Uint64(inodeBuf[92:100])

	return nil
}

func (fs *FileSystem) writeInode(inode *inode, idx int) error {
	// doesnt care about the free bitmap shit.
	// is a pure savage peak at binary serlaiztion

	inodeTableOffset := int(fs.superBlk.inodeTableStartBlock) * BLOCKSIZE
	startIdx := inodeTableOffset + idx*INODESIZE
	inodeBuf := make([]byte, INODESIZE) // 128

	binary.LittleEndian.PutUint32(inodeBuf[0:4], inode.size)
	inodeBuf[4] = byte(inode.fType)
	binary.LittleEndian.PutUint16(inodeBuf[5:7], inode.refs)

	for idx, val := range inode.owner {
		inodeBuf[7+idx] = val
	}

	if inode.setuid {
		inodeBuf[21] = 1
	} else {
		inodeBuf[21] = 0
	}

	inodeBuf[22] = byte(inode.ownerMode)
	inodeBuf[23] = byte(inode.otherMode)

	for idx, val := range inode.direct {
		off := idx * 4
		binary.LittleEndian.PutUint32(inodeBuf[24+off:28+off], val)
	}

	binary.LittleEndian.PutUint32(inodeBuf[72:76], inode.find)
	binary.LittleEndian.PutUint32(inodeBuf[76:80], inode.sind)
	binary.LittleEndian.PutUint32(inodeBuf[80:84], inode.tind)

	binary.LittleEndian.PutUint64(inodeBuf[84:92], inode.createdAt)
	binary.LittleEndian.PutUint64(inodeBuf[92:100], inode.modifiedAt)

	// ik the buffer is already zero'd but im simulating C kinda. Just keeping tidy and verbose.
	for i := 0; i < 28; i++ {
		inodeBuf[100+i] = 0
	}

	n, err := fs.disk.WriteAt(inodeBuf, int64(startIdx))
	if err != nil {
		return err
	}
	if n != INODESIZE {
		fmt.Println("WTF HAPPENED!!!! GRR")
		return io.ErrShortWrite
	}

	return nil
}

func (fs *FileSystem) Shutdown() {
	// close the file
	// this function is called on engine shutdown

	fs.disk.Close()
	return
}

func writeSuprBlktoSuprBuf(suprBuf []byte, suprBlk SuperBlock) {
	// Indexing is [inclusive]:[exclusive]
	copy(suprBuf[0:8], suprBlk.magic[:])
	binary.LittleEndian.PutUint32(suprBuf[8:12], suprBlk.version)
	binary.LittleEndian.PutUint32(suprBuf[12:16], suprBlk.blockSize)
	binary.LittleEndian.PutUint32(suprBuf[16:20], suprBlk.inodeCount)
	binary.LittleEndian.PutUint32(suprBuf[20:24], suprBlk.inodeSize)
	binary.LittleEndian.PutUint32(suprBuf[24:28], suprBlk.inodeTableStartBlock)
	binary.LittleEndian.PutUint32(suprBuf[28:32], suprBlk.inodeBitmapStartBlock)
	binary.LittleEndian.PutUint32(suprBuf[32:36], suprBlk.dataBlockCount)
	binary.LittleEndian.PutUint32(suprBuf[36:40], suprBlk.dataBlocksStartBlock)
	binary.LittleEndian.PutUint32(suprBuf[40:44], suprBlk.dataBitmapStartBlock)
	binary.LittleEndian.PutUint32(suprBuf[44:48], suprBlk.totalBlocks)
}
