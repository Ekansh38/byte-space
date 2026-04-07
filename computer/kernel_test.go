package computer

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func newPermKernel() *Kernel {
	memFs := afero.NewMemMapFs()
	c := &Computer{
		filesystem: memFs,
		FsMetaData: make(map[string]FileMetadata),
	}
	c.OS = &OS{Computer: c}
	return &Kernel{computer: c}
}

func TestResolvePath(t *testing.T) {
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
			kernel := &Kernel{}
			proc := &Process{CWD: tt.CWD, UID: tt.UID}

			result := kernel.resolvePath(proc, tt.path)

			assert.True(t, strings.HasPrefix(result, "/"))
			assert.NotContains(t, result, "..")
			assert.NotContains(t, result, "//")
			assert.Equal(t, tt.expected, result,
				"Expected result for resolvePath(%s, %s, %s) to be %s",
				tt.CWD, tt.UID, tt.path, tt.expected,
			)
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
				k.computer.filesystem.MkdirAll(dir, 0o755)
				k.computer.FsMetaData[dir] = FileMetadata{
					Filepath:  dir,
					Owner:     "alice",
					OwnerMode: tt.ownerMode,
					OtherMode: tt.otherMode,
				}
			}
			proc := &Process{CWD: "/", UID: tt.euid, EUID: tt.euid}

			err := k.ChangeDirectory(proc, dir)

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
			k.computer.filesystem.MkdirAll(dir, 0o755)
			k.computer.FsMetaData[dir] = FileMetadata{
				Filepath:  dir,
				Owner:     "alice",
				OwnerMode: tt.ownerMode,
				OtherMode: tt.otherMode,
			}
			proc := &Process{CWD: "/", UID: tt.euid, EUID: tt.euid}

			entries, err := k.ReadDir(proc, dir)

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
			k.computer.filesystem.MkdirAll("/home/alice", 0o755)
			afero.WriteFile(k.computer.filesystem, file, []byte(content), 0o644)
			k.computer.FsMetaData[file] = FileMetadata{
				Filepath:  file,
				Owner:     "alice",
				OwnerMode: tt.ownerMode,
				OtherMode: tt.otherMode,
			}
			proc := &Process{CWD: "/", UID: tt.euid, EUID: tt.euid}

			data, err := k.ReadFile(proc, file)

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
			k.computer.filesystem.MkdirAll(parent, 0o755)
			k.computer.FsMetaData[parent] = FileMetadata{
				Filepath:  parent,
				Owner:     "alice",
				OwnerMode: tt.ownerMode,
				OtherMode: tt.otherMode,
			}
			proc := &Process{CWD: "/", UID: tt.euid, EUID: tt.euid}

			err := k.MkDir(proc, newDir)

			if tt.wantErr == "" {
				assert.NoError(t, err)
				_, metaExists := k.computer.FsMetaData[newDir]
				assert.True(t, metaExists, "metadata should be created for new dir")
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
				_, metaExists := k.computer.FsMetaData[newDir]
				assert.False(t, metaExists, "metadata must not be created on failure")
			}
		})
	}
}
