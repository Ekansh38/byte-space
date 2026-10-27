package computer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newPermKernel() *Kernel {
	k := newTestKernel()
	// seed root with . and .. so ResolvePath has something to walk
	rootOps := &DirectoryOps{inodeNum: 2}
	rootOps.addDentry(k, DirEntry{Inum: 2, Name: "."})
	rootOps.addDentry(k, DirEntry{Inum: 2, Name: ".."})
	return k
}

// mkPermNode builds the parent chain as root (755) and the final node with
// the given owner + modes. paths is the chain, the last one is the node
// under test.
func mkPermNode(t *testing.T, k *Kernel, fType InodeType, owner string, ownerMode, otherMode uint8, names ...string) uint32 {
	t.Helper()
	parent := uint32(2)
	for i, name := range names {
		if i == len(names)-1 {
			inum, err := k.makeInode(parent, name, fType, owner, false, ownerMode, otherMode)
			if err != nil {
				t.Fatalf("makeInode failed: %v", err)
			}
			return inum
		}
		inum, err := k.makeInode(parent, name, S_IFDIR, "root", false, 7, 5)
		if err != nil {
			t.Fatalf("makeInode failed: %v", err)
		}
		parent = inum
	}
	return parent
}

func TestResolvePath(t *testing.T) {
	t.Skip("TODO(fs migration): resolvePath was merged into ResolvePath which now walks the inode tree and returns a uint32 inode num. Rewrite these string-normalization cases as pure path-string helper tests or against a fully initialized Kernel.")
	tests := []struct {
		path     string
		UID      string
		CWD      string
		expected string
	}{
		{
			path:     "~",
			UID:      "root",
			CWD:      "/",
			expected: "/root",
		},
		{
			path:     "./friends/dih",
			UID:      "boby",
			CWD:      "/root/bin/eggs",
			expected: "/root/bin/eggs/friends/dih",
		},
		{
			path:     "~",
			UID:      "alice",
			CWD:      "/random",
			expected: "/home/alice",
		},
		{
			path:     "~/docs",
			UID:      "alice",
			CWD:      "/random",
			expected: "/home/alice/docs",
		},
		{
			path:     "~/../bob",
			UID:      "alice",
			CWD:      "/random",
			expected: "/home/bob",
		},
		{
			path:     ".",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/home/user",
		},
		{
			path:     "./file",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/home/user/file",
		},
		{
			path:     "../file",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/home/file",
		},
		{
			path:     "../../file",
			UID:      "user",
			CWD:      "/home/user/docs",
			expected: "/home/file",
		},
		{
			path:     "../../../file",
			UID:      "user",
			CWD:      "/home/user/docs",
			expected: "/file", // clamp to root
		},
		{
			path:     "dir/file",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/home/user/dir/file",
		},
		{
			path:     "/absolute/path",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/absolute/path",
		},
		{
			path:     "/a/b/../c",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/a/c",
		},
		{
			path:     "/a/./b/./c",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/a/b/c",
		},
		{
			path:     "/a/b/c/..",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/a/b",
		},
		{
			path:     "////tmp///file",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/tmp/file",
		},
		{
			path:     "",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/home/user",
		},
		{
			path:     "./.././../file",
			UID:      "user",
			CWD:      "/a/b/c",
			expected: "/a/file",
		},
		{
			path:     "~/.hidden",
			UID:      "user",
			CWD:      "/tmp",
			expected: "/home/user/.hidden",
		},
		{
			path:     "../.hidden",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/home/.hidden",
		},
		{
			path:     "./.hidden",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/home/user/.hidden",
		},
		{
			path:     "../../../../../../etc",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/etc",
		},
		{
			path:     "~/../../../../etc",
			UID:      "user",
			CWD:      "/tmp",
			expected: "/etc",
		},
		{
			path:     "/../../../../etc",
			UID:      "user",
			CWD:      "/home/user",
			expected: "/etc",
		},
	}

	for _, tt := range tests {
		tt := tt // capture correctly

		t.Run("Testing Resolve Path", func(t *testing.T) {
			t.Skip("TODO(fs migration): rewritten later")
			kernel := &Kernel{}
			proc := &Process{CWD: tt.CWD, UID: tt.UID}

			_ = kernel
			_ = proc
			_ = tt
		})
	}
}


func TestChangeDirectoryPermissions(t *testing.T) {
	const dir = "/home/alice/secret"

	tests := []struct {
		name      string
		euid      string
		ownerMode uint8
		otherMode uint8
		mkDir     bool // whether to actually create the dir on the fs
		wantErr   string
	}{
		{
			name:      "owner with execute bit can cd",
			euid:      "alice",
			ownerMode: 1, // --x
			otherMode: 0,
			mkDir:     true,
		},
		{
			name:      "owner without execute bit is denied",
			euid:      "alice",
			ownerMode: 6, // rw-
			otherMode: 0,
			mkDir:     true,
			wantErr:   "permission denied",
		},
		{
			name:      "other user with other execute bit can cd",
			euid:      "bob",
			ownerMode: 0,
			otherMode: 1, // --x
			mkDir:     true,
		},
		{
			name:      "other user without other execute bit is denied",
			euid:      "bob",
			ownerMode: 7,
			otherMode: 0, // ---
			mkDir:     true,
			wantErr:   "permission denied",
		},
		{
			name:      "root bypasses permission check",
			euid:      "root",
			ownerMode: 0,
			otherMode: 0,
			mkDir:     true,
		},
		{
			name:    "directory does not exist returns error",
			euid:    "alice",
			mkDir:   false,
			wantErr: "no such file or directory",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			k := newPermKernel()
			if tt.mkDir {
				mkPermNode(t, k, S_IFDIR, "alice", tt.ownerMode, tt.otherMode, "home", "alice", "secret")
			}
			proc := &Process{CWD: "/", UID: tt.euid, EUID: tt.euid}

			_, err := k.Syscall(proc, SYS_CHDIR, dir)

			if tt.wantErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, dir, proc.CWD)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, "/", proc.CWD) // CWD must not change on failure
			}
		})
	}
}

func TestReadDirPermissions(t *testing.T) {
	const dir = "/home/alice/docs"

	tests := []struct {
		name      string
		euid      string
		ownerMode uint8
		otherMode uint8
		wantErr   string
	}{
		{
			name:      "owner with read bit can ls",
			euid:      "alice",
			ownerMode: 4, // r--
			otherMode: 0,
		},
		{
			name:      "owner without read bit is denied",
			euid:      "alice",
			ownerMode: 3, // -wx
			otherMode: 0,
			wantErr:   "permission denied",
		},
		{
			name:      "other user with other read bit can ls",
			euid:      "bob",
			ownerMode: 0,
			otherMode: 4, // r--
		},
		{
			name:      "other user without other read bit is denied",
			euid:      "bob",
			ownerMode: 7,
			otherMode: 0,
			wantErr:   "permission denied",
		},
		{
			name:      "root bypasses permission check",
			euid:      "root",
			ownerMode: 0,
			otherMode: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			k := newPermKernel()
			mkPermNode(t, k, S_IFDIR, "alice", tt.ownerMode, tt.otherMode, "home", "alice", "docs")
			proc := &Process{CWD: "/", UID: tt.euid, EUID: tt.euid}

			result, err := k.Syscall(proc, SYS_READDIR, dir)
			entries, _ := result.([]FileInfo)

			if tt.wantErr == "" {
				assert.NoError(t, err)
				assert.NotNil(t, entries)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, entries)
			}
		})
	}
}

func TestReadFilePermissions(t *testing.T) {
	const file = "/home/alice/secret.txt"
	const content = "top secret"

	tests := []struct {
		name      string
		euid      string
		ownerMode uint8
		otherMode uint8
		wantErr   string
	}{
		{
			name:      "owner with read bit can cat",
			euid:      "alice",
			ownerMode: 4, // r--
			otherMode: 0,
		},
		{
			name:      "owner without read bit is denied",
			euid:      "alice",
			ownerMode: 3, // -wx
			otherMode: 0,
			wantErr:   "permission denied",
		},
		{
			name:      "other user with other read bit can cat",
			euid:      "bob",
			ownerMode: 0,
			otherMode: 4, // r--
		},
		{
			name:      "other user without other read bit is denied",
			euid:      "bob",
			ownerMode: 7,
			otherMode: 0,
			wantErr:   "permission denied",
		},
		{
			name:      "root bypasses permission check",
			euid:      "root",
			ownerMode: 0,
			otherMode: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			k := newPermKernel()
			mkPermNode(t, k, S_IFREG, "alice", tt.ownerMode, tt.otherMode, "home", "alice", "secret.txt")
			// root writes the content, bypasses the perms under test
			rootProc := &Process{CWD: "/", UID: "root", EUID: "root"}
			if err := k.writeFile(rootProc, file, []byte(content)); err != nil {
				t.Fatalf("writeFile failed: %v", err)
			}
			proc := &Process{CWD: "/", UID: tt.euid, EUID: tt.euid}

			readResult, err := k.Syscall(proc, SYS_READ, file)
			data, _ := readResult.([]byte)

			if tt.wantErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, content, string(data))
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, data)
			}
		})
	}
}

func TestMkDirPermissions(t *testing.T) {
	const parent = "/home/alice"
	const newDir = "/home/alice/newdir"

	tests := []struct {
		name      string
		euid      string
		ownerMode uint8
		otherMode uint8
		wantErr   string
	}{
		{
			name:      "owner with write bit on parent can mkdir",
			euid:      "alice",
			ownerMode: 2, // -w-
			otherMode: 0,
		},
		{
			name:      "owner without write bit on parent is denied",
			euid:      "alice",
			ownerMode: 5, // r-x
			otherMode: 0,
			wantErr:   "permission denied",
		},
		{
			name:      "other user with other write bit on parent can mkdir",
			euid:      "bob",
			ownerMode: 0,
			otherMode: 2, // -w-
		},
		{
			name:      "other user without other write bit on parent is denied",
			euid:      "bob",
			ownerMode: 7,
			otherMode: 0,
			wantErr:   "permission denied",
		},
		{
			name:      "root bypasses permission check",
			euid:      "root",
			ownerMode: 0,
			otherMode: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			k := newPermKernel()
			mkPermNode(t, k, S_IFDIR, "alice", tt.ownerMode, tt.otherMode, "home", "alice")
			proc := &Process{CWD: "/", UID: tt.euid, EUID: tt.euid}

			_, err := k.Syscall(proc, SYS_MKDIR, newDir)

			rootProc := &Process{CWD: "/", UID: "root", EUID: "root"}
			if tt.wantErr == "" {
				assert.NoError(t, err)
				_, resolveErr := k.ResolvePath(rootProc, newDir)
				assert.NoError(t, resolveErr, "new dir should exist on disk")
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
				_, resolveErr := k.ResolvePath(rootProc, newDir)
				assert.Error(t, resolveErr, "dir must not be created on failure")
			}
		})
	}
}
