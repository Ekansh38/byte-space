package computer

import (
	"encoding/binary"
	"testing"
	"time"
)

// NewTestFileSystem mirrors NewFileSystem but uses an in-memory MemDisk
// instead of a real file on disk. Useful for unit tests that need a fully
// formatted filesystem without touching the host filesystem.
func NewTestFileSystem() *FileSystem {
	disk := &MemDisk{buf: make([]byte, DISKSIZE)}

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

	superBuf := make([]byte, BLOCKSIZE)
	writeSuprBlktoSuprBuf(superBuf, superBlk)
	_, _ = disk.WriteAt(superBuf, 0)

	// inode bitmap: mark inodes 0,1,2 as taken (2 is root)
	inodeBitmapBuf := make([]byte, BLOCKSIZE)
	inodeBitmapBuf[0] = 0b00000111
	_, _ = disk.WriteAt(inodeBitmapBuf, BLOCKSIZE*int64(superBlk.inodeBitmapStartBlock))

	// inode table (zeroed)
	inodeTableBuf := make([]byte, BLOCKSIZE*252)
	_, _ = disk.WriteAt(inodeTableBuf, BLOCKSIZE*int64(superBlk.inodeTableStartBlock))

	// data bitmap (zeroed)
	dataBitmapBuf := make([]byte, BLOCKSIZE)
	_, _ = disk.WriteAt(dataBitmapBuf, int64(superBlk.dataBitmapStartBlock)*BLOCKSIZE)

	// data blocks (zeroed)
	dataBlocksBuf := make([]byte, BLOCKSIZE*superBlk.dataBlockCount)
	_, _ = disk.WriteAt(dataBlocksBuf, int64(superBlk.dataBlocksStartBlock)*BLOCKSIZE)

	fs := &FileSystem{
		superBlk: superBlk,
		disk:     disk,
	}

	// create the root inode
	fs.writeInode(&inode{
		size:       0,
		fType:      S_IFDIR,
		refs:       0,
		owner:      [14]byte{'r', 'o', 'o', 't'},
		setuid:     false,
		ownerMode:  0b111,
		otherMode:  0b101,
		direct:     [12]uint32{},
		find:       0,
		sind:       0,
		tind:       0,
		createdAt:  uint64(time.Now().Unix()),
		modifiedAt: uint64(time.Now().Unix()),
	}, 2)

	return fs
}

func TestNewTestFileSystemSuperBlock(t *testing.T) {
	fs := NewTestFileSystem()

	if string(fs.superBlk.magic[:MAGICLEN]) != "BS-EXTFS" {
		t.Errorf("bad magic: %s", fs.superBlk.magic)
	}
	if fs.superBlk.version != LATEST_VERSION {
		t.Errorf("bad version: %d", fs.superBlk.version)
	}
}

func TestReadInode(t *testing.T) {
	fs := NewTestFileSystem()

	tests := []struct {
		name    string
		setup   func() *inode
		idx     uint32
		want    *inode
		wantErr bool
	}{
		{
			name: "root inode read back",
			setup: func() *inode {
				return &inode{}
			},
			idx: 2,
			want: &inode{
				size:       0,
				fType:      S_IFDIR,
				refs:       0,
				owner:      [14]byte{'r', 'o', 'o', 't'},
				setuid:     false,
				ownerMode:  0b111,
				otherMode:  0b101,
				direct:     [12]uint32{},
				find:       0,
				sind:       0,
				tind:       0,
				num:        2,
			},
			wantErr: false,
		},
		{
			name: "freshly zeroed inode",
			setup: func() *inode {
				return &inode{}
			},
			idx:     3,
			want:    &inode{num: 3},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.setup()
			err := fs.readInode(in, tt.idx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected err=%v, got %v", tt.wantErr, err)
			}
			if tt.want == nil {
				return
			}
			if in.size != tt.want.size {
				t.Errorf("size: expected %d, got %d", tt.want.size, in.size)
			}
			if in.fType != tt.want.fType {
				t.Errorf("fType: expected %d, got %d", tt.want.fType, in.fType)
			}
			if in.refs != tt.want.refs {
				t.Errorf("refs: expected %d, got %d", tt.want.refs, in.refs)
			}
			if in.owner != tt.want.owner {
				t.Errorf("owner: expected %v, got %v", tt.want.owner, in.owner)
			}
			if in.setuid != tt.want.setuid {
				t.Errorf("setuid: expected %v, got %v", tt.want.setuid, in.setuid)
			}
			if in.ownerMode != tt.want.ownerMode {
				t.Errorf("ownerMode: expected %d, got %d", tt.want.ownerMode, in.ownerMode)
			}
			if in.otherMode != tt.want.otherMode {
				t.Errorf("otherMode: expected %d, got %d", tt.want.otherMode, in.otherMode)
			}
			if in.direct != tt.want.direct {
				t.Errorf("direct: expected %v, got %v", tt.want.direct, in.direct)
			}
			if in.find != tt.want.find {
				t.Errorf("find: expected %d, got %d", tt.want.find, in.find)
			}
			if in.sind != tt.want.sind {
				t.Errorf("sind: expected %d, got %d", tt.want.sind, in.sind)
			}
			if in.tind != tt.want.tind {
				t.Errorf("tind: expected %d, got %d", tt.want.tind, in.tind)
			}
			if in.num != tt.want.num {
				t.Errorf("num: expected %d, got %d", tt.want.num, in.num)
			}
		})
	}
}

func TestWriteInode(t *testing.T) {
	tests := []struct {
		name    string
		in      *inode
		idx     uint32
		wantErr bool
	}{
		{
			name: "write a regular file inode",
			in: &inode{
				size:      4096,
				fType:     S_IFREG,
				refs:      1,
				owner:     [14]byte{'e', 'k', 'a', 'n', 's', 'h'},
				setuid:    true,
				ownerMode: 0b111,
				otherMode: 0b100,
				direct: [12]uint32{
					1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
				},
				find:       13,
				sind:       14,
				tind:       15,
				createdAt:  1700000000,
				modifiedAt: 1700000001,
			},
			idx:     4,
			wantErr: false,
		},
		{
			name: "write a directory inode",
			in: &inode{
				size:       0,
				fType:      S_IFDIR,
				refs:       2,
				owner:      [14]byte{'r', 'o', 'o', 't'},
				setuid:     false,
				ownerMode:  0b111,
				otherMode:  0b101,
				direct:     [12]uint32{},
				find:       0,
				sind:       0,
				tind:       0,
				createdAt:  1700000002,
				modifiedAt: 1700000003,
			},
			idx:     5,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewTestFileSystem()

			err := fs.writeInode(tt.in, tt.idx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected err=%v, got %v", tt.wantErr, err)
			}

			got := &inode{}
			if err := fs.readInode(got, tt.idx); err != nil {
				t.Fatalf("readInode failed: %v", err)
			}

			if got.size != tt.in.size {
				t.Errorf("size: expected %d, got %d", tt.in.size, got.size)
			}
			if got.fType != tt.in.fType {
				t.Errorf("fType: expected %d, got %d", tt.in.fType, got.fType)
			}
			if got.refs != tt.in.refs {
				t.Errorf("refs: expected %d, got %d", tt.in.refs, got.refs)
			}
			if got.owner != tt.in.owner {
				t.Errorf("owner: expected %v, got %v", tt.in.owner, got.owner)
			}
			if got.setuid != tt.in.setuid {
				t.Errorf("setuid: expected %v, got %v", tt.in.setuid, got.setuid)
			}
			if got.ownerMode != tt.in.ownerMode {
				t.Errorf("ownerMode: expected %d, got %d", tt.in.ownerMode, got.ownerMode)
			}
			if got.otherMode != tt.in.otherMode {
				t.Errorf("otherMode: expected %d, got %d", tt.in.otherMode, got.otherMode)
			}
			if got.direct != tt.in.direct {
				t.Errorf("direct: expected %v, got %v", tt.in.direct, got.direct)
			}
			if got.find != tt.in.find {
				t.Errorf("find: expected %d, got %d", tt.in.find, got.find)
			}
			if got.sind != tt.in.sind {
				t.Errorf("sind: expected %d, got %d", tt.in.sind, got.sind)
			}
			if got.tind != tt.in.tind {
				t.Errorf("tind: expected %d, got %d", tt.in.tind, got.tind)
			}
			if got.createdAt != tt.in.createdAt {
				t.Errorf("createdAt: expected %d, got %d", tt.in.createdAt, got.createdAt)
			}
			if got.modifiedAt != tt.in.modifiedAt {
				t.Errorf("modifiedAt: expected %d, got %d", tt.in.modifiedAt, got.modifiedAt)
			}
			if got.num != tt.idx {
				t.Errorf("num: expected %d, got %d", tt.idx, got.num)
			}
		})
	}
}

func TestFalloc(t *testing.T) {
	setBit := func(bm []byte, idx uint32) {
		bm[idx/8] |= 1 << (idx % 8)
	}
	bitSet := func(bm []byte, idx uint32) bool {
		return bm[idx/8]&(1<<(idx%8)) != 0
	}

	// readDataBitmap reads the current data bitmap from disk.
	readDataBitmap := func(fs *FileSystem) []byte {
		bm := make([]byte, BLOCKSIZE)
		_, _ = fs.disk.ReadAt(bm, int64(fs.superBlk.dataBitmapStartBlock)*BLOCKSIZE)
		return bm
	}

	// writeFindBlock writes the indirect table block contents for the given
	// indirect block address (a physical data block index).
	writeIndirectBlock := func(fs *FileSystem, indirectBlk uint32, entries []uint32) {
		buf := make([]byte, BLOCKSIZE)
		for i, v := range entries {
			binary.LittleEndian.PutUint32(buf[i*4:i*4+4], v)
		}
		off := int64(fs.superBlk.dataBlocksStartBlock)*BLOCKSIZE + int64(indirectBlk)*BLOCKSIZE
		_, _ = fs.disk.WriteAt(buf, off)
	}

	tests := []struct {
		name string
		// setup returns the inode to falloc against and pre-seeds the disk
		// (data bitmap + any indirect blocks).
		setup    func(fs *FileSystem) *inode
		newSize  uint32
		wantErr  bool
		// wantDirect is the expected direct array after falloc runs.
		wantDirect [12]uint32
		// blocks that must be marked FREE in the data bitmap afterwards.
		wantFree []uint32
		// blocks that must remain ALLOCATED in the data bitmap afterwards.
		wantUsed []uint32
	}{
		{
			name: "no-op when block count is unchanged",
			setup: func(fs *FileSystem) *inode {
				bm := make([]byte, BLOCKSIZE)
				setBit(bm, 10)
				setBit(bm, 11)
				setBit(bm, 12)
				_, _ = fs.disk.WriteAt(bm, int64(fs.superBlk.dataBitmapStartBlock)*BLOCKSIZE)

				return &inode{
					size:   3 * BLOCKSIZE,
					fType:  S_IFREG,
					direct: [12]uint32{10, 11, 12, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				}
			},
			newSize:     2*BLOCKSIZE + 1, // still 3 blocks -> no work
			wantDirect:  [12]uint32{10, 11, 12, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			wantUsed:    []uint32{10, 11, 12},
		},
		{
			name: "shrink within direct blocks frees tail blocks",
			setup: func(fs *FileSystem) *inode {
				bm := make([]byte, BLOCKSIZE)
				for _, b := range []uint32{10, 11, 12, 13, 14} {
					setBit(bm, b)
				}
				_, _ = fs.disk.WriteAt(bm, int64(fs.superBlk.dataBitmapStartBlock)*BLOCKSIZE)

				return &inode{
					size:   5 * BLOCKSIZE,
					fType:  S_IFREG,
					direct: [12]uint32{10, 11, 12, 13, 14, 0, 0, 0, 0, 0, 0, 0},
				}
			},
			newSize:    2 * BLOCKSIZE, // 2 blocks needed -> free direct[2..4]
			wantDirect: [12]uint32{10, 11, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			wantFree:   []uint32{12, 13, 14},
			wantUsed:   []uint32{10, 11},
		},
		{
			name: "shrink across direct/find boundary frees indirect block",
			setup: func(fs *FileSystem) *inode {
				// 13 blocks: 12 direct + 1 in find.
				bm := make([]byte, BLOCKSIZE)
				// direct data blocks 20..31
				for b := uint32(20); b <= 31; b++ {
					setBit(bm, b)
				}
				// indirect table lives in data block 32, the 13th data block is 33.
				setBit(bm, 32)
				setBit(bm, 33)
				_, _ = fs.disk.WriteAt(bm, int64(fs.superBlk.dataBitmapStartBlock)*BLOCKSIZE)

				// seed the find indirect block (block 32) -> entry 0 points at block 33
				writeIndirectBlock(fs, 32, []uint32{33})

				return &inode{
					size:   13 * BLOCKSIZE,
					fType:  S_IFREG,
					direct: [12]uint32{20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
					find:   32,
				}
			},
			newSize:    11 * BLOCKSIZE, // 11 blocks needed -> free direct[11] + find[0] + find block itself
			wantDirect: [12]uint32{20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 0},
			wantFree:   []uint32{31, 32, 33},
			wantUsed:   []uint32{20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewTestFileSystem()
			in := tt.setup(fs)

			err := fs.Falloc(in, tt.newSize)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected err=%v, got %v", tt.wantErr, err)
			}

			if in.direct != tt.wantDirect {
				t.Errorf("direct: expected %v, got %v", tt.wantDirect, in.direct)
			}

			bm := readDataBitmap(fs)
			for _, b := range tt.wantFree {
				if bitSet(bm, b) {
					t.Errorf("expected block %d to be free, but it is still allocated", b)
				}
			}
			for _, b := range tt.wantUsed {
				if !bitSet(bm, b) {
					t.Errorf("expected block %d to remain allocated, but it was freed", b)
				}
			}
		})
	}
}
