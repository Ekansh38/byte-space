package computer

// 3. DATA BLOCK HELPERS  (fs.go, alongside ReadBlock/WriteBlock)
//    - readDataBlock(dataBlkIdx uint32) []byte — takes a *data-region-relative*
//      index (the values stored in inode.direct / find / sind / tind),
//      converts to absolute (dataBlkIdx + dataBlocksStartBlock), returns buf.
//    - writeDataBlock(dataBlkIdx uint32, data []byte).
//    - lots of stuff below needs these: dir ops, dir adder, FileFD, etc.

//DONE

//
// 4. DIRECTORYOPS + InodeOperations impl  (fs_inode.go / fs_dir.go)
//    - resurrect the commented-out DirectoryOps struct in fs_inode.go.
//    - ReadEntries(k *Kernel): walk inode.direct[] (later find/sind/tind too),
//      readDataBlock each, decodeDirEntries, concat the slices.
//    - CreateFD(...): returns a DirFD (see step 7). ok to stub for now.
//    - central place: whenever an inode is loaded, if fType==S_IFDIR set
//      ops = &DirectoryOps{}, else &FileOps{} (once that exists). do this in
//      a getInode(inum) helper so it's in ONE place in the kernel btw.
//
//
// 5. DIR ENTRY ADD / REMOVE  (fs_dir.go)
//    - addDirEntry(dir *inode, name string, inum uint32) error:
//      walk dir's blocks, find the first 64-byte slot with inum==0, encode,
//      writeDataBlock. if no slot, Falloc(dir, dir.size + BLOCKSIZE), grab
//      the new block, retry.
//    - removeDirEntry(dir *inode, name string) error: walk, match by name,
//      zero the 64 bytes, writeDataBlock. leave block allocated for now —
//      shrinking on empty tail block is a later optimization.
//
// 6. FIX ResolvePath  (kernel.go:151 — currently marked "FULL OF BUGS")
//    - strings.Split("/a/b", "/") gives ["", "a", "b"] filter empties.  DONE
//    - dirs[0] never advances in the loop today, so infinite loop.
//    - on cache miss `mostRecentInode` is nil BEFORE readInode is called —
//      allocate an &inode{} first, then read into it.
//    - loaded inodes never get their .ops set — do it right after readInode
//      based on fType (dir vs regular).
//    - if a non-dir shows up mid-path, return 0 / ENOTDIR.
//    - factor getInode(inum) *inode so cache lookup + load + ops-assign is
//      in one place, then the walk loop is a lot shorter.
//
// 7. FD TYPES  (new fs_fd.go)
//    - FileFD{inum, offset, flags}:
//        Read: which virtual block does offset land in? virtualToPhysical
//        to jump into direct/find/sind/tind, readDataBlock, slice, advance.
//        Write: mirror. if past end, Falloc first, then writeDataBlock.
//    - DirFD{inum, cursor}: Read returns raw 64-byte entries, or expose
//      ReadEntries directly and skip byte-level Read.
//    - the existing kernel FileDescription struct will get replaced by
//      these FD implementations — but keep both alive during migration.
//
// 8. ROOT DIR + INITIAL FS TREE
//    - after a fresh format, root inode has zero entries. add "." and ".."
//      pointing to inode 2 (both point at itself for root).
//    - port initFileSystem from computer.go into a first-boot routine that
//      uses the new syscalls: mkdir /etc /bin /home /var /var/log /tmp,
//      create /etc/passwd, /etc/hostname, /etc/issue, /etc/motd, /bin/*.
//    - only run when isInitialized == false in NewFileSystem.
//
// 9. MIGRATE SYSCALLS OFF AFERO  (kernel.go, os.go)
//    - readFile / writeFile / mkDir / createFile / removeAll / stat / chmod /
//      changeDirectory all currently call k.computer.OS.* (afero). switch
//      them one at a time: ResolvePath -> getInode -> do the thing.
//    - canRead/canWrite/canExecute currently look up FsMetaData by path —
//      switch them to look at the inode's owner + ownerMode + otherMode.
//      once nothing references FsMetaData, delete the JSON file + loaders.
//    - once nothing calls k.computer.filesystem or k.computer.OS.*, delete
//      the afero field, os.go, populateFileMetadata, initFileSystem.
//
// 10. INODE CACHE WRITE-BACK
//    - inode.dirty is set (e.g. in Falloc) but nobody ever flushes it.
//    - add fs.SyncInode(*inode) and fs.SyncAll() that walks the kernel
//      inodeCache and writeInode's any dirty ones. call from Shutdown, and
//      later maybe from a periodic goroutine.
//
// LATER (not blocking anything above):
//   - procfs / virtual inodes (10000+, 20000+ ranges — see FILESYSTEM.md)
//   - shrinking dir blocks when trailing block is empty
//   - concurrency audit of fs.mu vs kernel.fsMu (currently overlapping)
//   - inodeCache eviction (grows forever right now)

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type MemDisk struct {
	buf []byte
}

func (m *MemDisk) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("Negative offset")
	}
	if off >= int64(len(m.buf)) {
		return 0, io.EOF
	}
	n = copy(p, m.buf[off:])
	if n < len(p) {
		err = io.EOF
	}
	return n, err
}

func (m *MemDisk) WriteAt(b []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("Negative offset")
	}
	if off >= int64(len(m.buf)) {
		if len(b) == 0 {
			return 0, nil
		}
		return 0, io.ErrShortWrite
	}

	n = copy(m.buf[off:], b)

	if n < len(b) {
		return n, io.ErrShortWrite
	}

	return len(b), nil
}

func (m *MemDisk) Truncate(size int64) error {
	if size < 0 {
		return errors.New("Negative size")
	}
	if size < int64(len(m.buf)) {
		m.buf = m.buf[:size]
	} else if size > int64(len(m.buf)) {
		extention := make([]byte, size-int64(len(m.buf)))
		m.buf = append(m.buf, extention...)
	}
	return nil
}

func (m *MemDisk) Close() error {
	return nil
}

type Disk interface {
	ReadAt(p []byte, off int64) (n int, err error)
	WriteAt(b []byte, off int64) (n int, err error)
	Truncate(size int64) error
	Close() error
}

// PLANNING
// if string too short ["p", "i", "c", "s", "\0","0","0","0"] // the extra 0's are padding

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
	disk          Disk
	superBlk      SuperBlock // cached
	inodeBitmapMu sync.Mutex
	mu            sync.Mutex

	// maybe later cache the bitmaps for extra SPEED.
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

	if err := disk.Truncate(DISKSIZE); err != nil {
		panic(err)
	}

	// now check for the superblk header being correct and up to date.

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
		log.Printf("Invalid magic: expected BS-EXTFS, got %s", headerSuperBlk.magic)
		isInitialized = false
	}
	if headerSuperBlk.version != LATEST_VERSION && isInitialized == true {
		log.Printf("Invalid version: expected %d, got %d", LATEST_VERSION, headerSuperBlk.version)
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
		// We have fewer data blocks than the bitmap can hold (16128 vs 32768 bits).
		dataBitmapUsedBytes := int(superBlk.dataBlockCount) / 8
		for idx := dataBitmapUsedBytes; idx < BLOCKSIZE; idx++ {
			dataBitmapBuf[idx] = 0xFF
		}
		disk.WriteAt(dataBitmapBuf, int64(superBlk.dataBitmapStartBlock)*BLOCKSIZE)
		disk.WriteAt(dataBitmapBuf, int64(superBlk.dataBitmapStartBlock)*BLOCKSIZE)

		// data blocks
		dataBlocksBuf := make([]byte, BLOCKSIZE*superBlk.dataBlockCount)
		disk.WriteAt(dataBlocksBuf, int64(superBlk.dataBlocksStartBlock)*BLOCKSIZE)

		// padding

		paddingBuf := make([]byte, BLOCKSIZE)
		for idx := range paddingBuf {
			paddingBuf[idx] = 103 // 0x67
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

func (fs *FileSystem) Shutdown() {
	// close the file
	// this function is called on engine shutdown

	fs.disk.Close()
}

// TODO: think about concurrency with these things in the future.

func (fs *FileSystem) ReadBlock(blockNum uint32) ([]byte, error) { // NOT DATA BLOCK! ANY BLOCK

	if blockNum >= fs.superBlk.totalBlocks {
		return nil, errors.New("invalid blockNum")
	}

	offset := int64(blockNum) * int64(BLOCKSIZE)

	block := make([]byte, BLOCKSIZE)
	_, err := fs.disk.ReadAt(block, offset)

	return block, err
}

func (fs *FileSystem) WriteBlock(blockNum uint32, data []byte) error { // NOT DATA BLOCK! ANY BLOCK

	if blockNum >= fs.superBlk.totalBlocks {
		return errors.New("invalid blockNum")
	}

	if len(data) != BLOCKSIZE {
		return errors.New("len(data) != BLOCKSIZE")
	}

	offset := int64(blockNum) * int64(BLOCKSIZE)

	_, err := fs.disk.WriteAt(data, offset)

	return err
}

// readDataBlock reads a block from the data region. dataBlkIdx is the
// data-region-relative index (i.e. the values stored in inode.direct /
// find / sind / tind, and in the data bitmap).
func (fs *FileSystem) readDataBlock(dataBlkIdx uint32) ([]byte, error) {
	if dataBlkIdx >= fs.superBlk.dataBlockCount {
		return nil, errors.New("invalid dataBlkIdx")
	}
	return fs.ReadBlock(dataBlkIdx + fs.superBlk.dataBlocksStartBlock)
}

func (fs *FileSystem) writeDataBlock(dataBlkIdx uint32, data []byte) error {
	if dataBlkIdx >= fs.superBlk.dataBlockCount {
		return errors.New("invalid dataBlkIdx")
	}
	return fs.WriteBlock(dataBlkIdx+fs.superBlk.dataBlocksStartBlock, data)
}
