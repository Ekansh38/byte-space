package computer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
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
		// wantSize is the expected inode.size after falloc; this is the
		// authoritative bound — the system reads based on size, not zero-checks.
		wantSize uint32
		// wantDirect is the expected direct array after falloc runs.
		// Deallocation does NOT zero block pointers; stale values remain in
		// freed slots and are simply unreachable via inode.size.
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
			newSize:    2*BLOCKSIZE + 1, // still 3 blocks -> no work
			wantSize:   2*BLOCKSIZE + 1,
			wantDirect: [12]uint32{10, 11, 12, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			wantUsed:   []uint32{10, 11, 12},
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
			newSize:  2 * BLOCKSIZE,
			wantSize: 2 * BLOCKSIZE,
			// Freed slots (direct[2..4]) retain stale block numbers; they are
			// unreachable because inode.size == 2*BLOCKSIZE caps valid blocks at 2.
			wantDirect: [12]uint32{10, 11, 12, 13, 14, 0, 0, 0, 0, 0, 0, 0},
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
			newSize:  11 * BLOCKSIZE,
			wantSize: 11 * BLOCKSIZE,
			// direct[11] retains its stale pointer (31); inode.find retains 32.
			// Both are unreachable: inode.size == 11*BLOCKSIZE caps valid blocks at 11.
			wantDirect: [12]uint32{20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
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

			if in.size != tt.wantSize {
				t.Errorf("size: expected %d, got %d", tt.wantSize, in.size)
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

// newTestKernel wires a Kernel around NewTestFileSystem so ops methods
// (which go through kernel.computer.fs) can run against the MemDisk.
func newTestKernel() *Kernel {
	return &Kernel{
		computer:   &Computer{fs: NewTestFileSystem()},
		inodeCache: make(map[uint32]*inode),
	}
}

func TestAddDentry(t *testing.T) {
	tests := []struct {
		name string
		// add returns the dentries to add in order; the last add is the one
		// under test when wantErr is set.
		add      []DirEntry
		wantErr  bool
		wantSize uint32 // expected dir inode size after all adds
	}{
		{
			name:     "first entry fallocs the first block",
			add:      []DirEntry{{Inum: 2, Name: "."}},
			wantSize: BLOCKSIZE,
		},
		{
			name:     "second entry reuses a slot in the same block",
			add:      []DirEntry{{Inum: 2, Name: "."}, {Inum: 2, Name: ".."}},
			wantSize: BLOCKSIZE,
		},
		{
			name: "65th entry fallocs a second block",
			add: func() []DirEntry {
				var entries []DirEntry
				for i := 0; i < 65; i++ {
					entries = append(entries, DirEntry{Inum: uint32(i + 3), Name: fmt.Sprintf("file-%d", i)})
				}
				return entries
			}(),
			wantSize: 2 * BLOCKSIZE,
		},
		{
			name:    "name too long errors",
			add:     []DirEntry{{Inum: 3, Name: strings.Repeat("a", 56)}},
			wantErr: true,
		},
		{
			name:    "inum 0 errors",
			add:     []DirEntry{{Inum: 0, Name: "ghost"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := newTestKernel()
			ops := &DirectoryOps{inodeNum: 2}

			var err error
			for _, dentry := range tt.add {
				err = ops.addDentry(k, dentry)
				if err != nil {
					break
				}
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected err=%v, got %v", tt.wantErr, err)
			}
			if tt.wantErr {
				return
			}

			dirInode := &inode{}
			if err := k.computer.fs.readInode(dirInode, 2); err != nil {
				t.Fatalf("readInode failed: %v", err)
			}
			if dirInode.size != tt.wantSize {
				t.Errorf("size: expected %d, got %d", tt.wantSize, dirInode.size)
			}

			entries, err := ops.ReadEntries(k)
			if err != nil {
				t.Fatalf("ReadEntries failed: %v", err)
			}
			if len(entries) != len(tt.add) {
				t.Fatalf("entries: expected %d, got %d", len(tt.add), len(entries))
			}
			for i := range tt.add {
				if entries[i] != tt.add[i] {
					t.Errorf("entry %d: expected %+v, got %+v", i, tt.add[i], entries[i])
				}
			}
		})
	}
}

func TestRemoveDentry(t *testing.T) {
	k := newTestKernel()
	ops := &DirectoryOps{inodeNum: 2}

	for _, dentry := range []DirEntry{{Inum: 3, Name: "keep"}, {Inum: 4, Name: "gone"}} {
		if err := ops.addDentry(k, dentry); err != nil {
			t.Fatalf("addDentry failed: %v", err)
		}
	}

	if err := ops.removeDentry(k, "gone"); err != nil {
		t.Fatalf("removeDentry failed: %v", err)
	}

	entries, err := ops.ReadEntries(k)
	if err != nil {
		t.Fatalf("ReadEntries failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "keep" {
		t.Fatalf("expected only 'keep' left, got %+v", entries)
	}

	// removing something that isn't there
	if err := ops.removeDentry(k, "gone"); err != ENOENT {
		t.Errorf("expected ENOENT, got %v", err)
	}

	// the freed slot gets reused, no new block falloc'd
	if err := ops.addDentry(k, DirEntry{Inum: 5, Name: "reuse"}); err != nil {
		t.Fatalf("addDentry failed: %v", err)
	}
	dirInode := &inode{}
	if err := k.computer.fs.readInode(dirInode, 2); err != nil {
		t.Fatalf("readInode failed: %v", err)
	}
	if dirInode.size != BLOCKSIZE {
		t.Errorf("size: expected %d, got %d", BLOCKSIZE, dirInode.size)
	}
}

// mkTestDir allocates a dir inode, links it into the parent and seeds . / ..
func mkTestDir(t *testing.T, k *Kernel, parentInum uint32, name string) uint32 {
	t.Helper()
	inum, err := k.computer.fs.AllocInode()
	if err != nil {
		t.Fatalf("AllocInode failed: %v", err)
	}
	if err := k.computer.fs.writeInode(&inode{
		fType:     S_IFDIR,
		owner:     [14]byte{'r', 'o', 'o', 't'},
		ownerMode: 0b111,
		otherMode: 0b101,
	}, inum); err != nil {
		t.Fatalf("writeInode failed: %v", err)
	}

	parentOps := &DirectoryOps{inodeNum: parentInum}
	if err := parentOps.addDentry(k, DirEntry{Inum: inum, Name: name}); err != nil {
		t.Fatalf("addDentry failed: %v", err)
	}

	ops := &DirectoryOps{inodeNum: inum}
	if err := ops.addDentry(k, DirEntry{Inum: inum, Name: "."}); err != nil {
		t.Fatalf("addDentry . failed: %v", err)
	}
	if err := ops.addDentry(k, DirEntry{Inum: parentInum, Name: ".."}); err != nil {
		t.Fatalf("addDentry .. failed: %v", err)
	}
	return inum
}

// mkTestFile allocates a file inode and links it into the parent.
func mkTestFile(t *testing.T, k *Kernel, parentInum uint32, name string) uint32 {
	t.Helper()
	inum, err := k.computer.fs.AllocInode()
	if err != nil {
		t.Fatalf("AllocInode failed: %v", err)
	}
	if err := k.computer.fs.writeInode(&inode{
		fType:     S_IFREG,
		owner:     [14]byte{'r', 'o', 'o', 't'},
		ownerMode: 0b111,
		otherMode: 0b101,
	}, inum); err != nil {
		t.Fatalf("writeInode failed: %v", err)
	}
	parentOps := &DirectoryOps{inodeNum: parentInum}
	if err := parentOps.addDentry(k, DirEntry{Inum: inum, Name: name}); err != nil {
		t.Fatalf("addDentry failed: %v", err)
	}
	return inum
}

func TestResolvePathWalk(t *testing.T) {
	k := newTestKernel()

	// seed root with . and ..
	rootOps := &DirectoryOps{inodeNum: 2}
	if err := rootOps.addDentry(k, DirEntry{Inum: 2, Name: "."}); err != nil {
		t.Fatalf("addDentry failed: %v", err)
	}
	if err := rootOps.addDentry(k, DirEntry{Inum: 2, Name: ".."}); err != nil {
		t.Fatalf("addDentry failed: %v", err)
	}

	etc := mkTestDir(t, k, 2, "etc")
	passwd := mkTestFile(t, k, etc, "passwd")
	home := mkTestDir(t, k, 2, "home")
	alice := mkTestDir(t, k, home, "alice")

	tests := []struct {
		name    string
		path    string
		cwd     string
		uid     string
		want    uint32
		wantErr error
	}{
		{name: "root", path: "/", cwd: "/", uid: "root", want: 2},
		{name: "abs dir", path: "/etc", cwd: "/", uid: "root", want: etc},
		{name: "abs file", path: "/etc/passwd", cwd: "/", uid: "root", want: passwd},
		{name: "relative from root", path: "etc/passwd", cwd: "/", uid: "root", want: passwd},
		{name: "relative from etc", path: "passwd", cwd: "/etc", uid: "root", want: passwd},
		{name: "dotdot in path", path: "../etc/passwd", cwd: "/home", uid: "root", want: passwd},
		{name: "tilde expansion", path: "~", cwd: "/", uid: "alice", want: alice},
		{name: "empty means cwd", path: "", cwd: "/etc", uid: "root", want: etc},
		{name: "missing entry", path: "/nope", cwd: "/", uid: "root", wantErr: ENOENT},
		{name: "file mid path", path: "/etc/passwd/deeper", cwd: "/", uid: "root", wantErr: ENOTDIR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := &Process{CWD: tt.cwd, UID: tt.uid, EUID: tt.uid}
			got, err := k.ResolvePath(proc, tt.path)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("expected err %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected inum %d, got %d", tt.want, got)
			}
		})
	}
}

func TestFileFDReadWrite(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "small write within one block", data: []byte("hello bs-extfs")},
		{name: "write across a block boundary", data: bytes.Repeat([]byte("x"), BLOCKSIZE+100)},
		{name: "write into the find indirection", data: bytes.Repeat([]byte("y"), 13*BLOCKSIZE)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := newTestKernel()
			inum := mkTestFile(t, k, 2, "testfile")

			in, err := k.getInode(inum)
			if err != nil {
				t.Fatalf("getInode failed: %v", err)
			}

			wfd := in.ops.CreateFD(k, inum, "/testfile", 0)
			if err := wfd.Open(); err != nil {
				t.Fatalf("Open failed: %v", err)
			}
			n, err := wfd.Write(tt.data)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
			if n != len(tt.data) {
				t.Fatalf("Write: expected %d bytes, got %d", len(tt.data), n)
			}

			// fresh fd, fresh offset — read it all back
			rfd := in.ops.CreateFD(k, inum, "/testfile", 0)
			buf := make([]byte, len(tt.data))
			n, err = rfd.Read(buf)
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			if n != len(tt.data) {
				t.Fatalf("Read: expected %d bytes, got %d", len(tt.data), n)
			}
			if !bytes.Equal(buf, tt.data) {
				t.Errorf("read back data does not match written data")
			}

			// reading past the end gives EOF
			if _, err := rfd.Read(buf); err != io.EOF {
				t.Errorf("expected io.EOF, got %v", err)
			}
		})
	}
}

func TestFileFDOverwrite(t *testing.T) {
	k := newTestKernel()
	inum := mkTestFile(t, k, 2, "notes")

	in, _ := k.getInode(inum)

	fd := in.ops.CreateFD(k, inum, "/notes", 0)
	if _, err := fd.Write([]byte("aaaaaaaaaa")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// overwrite the middle without growing
	fd.SetOffset(3)
	if _, err := fd.Write([]byte("BBB")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	rfd := in.ops.CreateFD(k, inum, "/notes", 0)
	buf := make([]byte, 10)
	if _, err := rfd.Read(buf); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(buf) != "aaaBBBaaaa" {
		t.Errorf("expected aaaBBBaaaa, got %s", buf)
	}
}
