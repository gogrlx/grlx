// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package procfs

import (
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/fd"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/linux"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal/testutils"
)

func newPrivateProcMountSubset() (*Handle, error)   { return newPrivateProcMount(true) }
func newPrivateProcMountUnmasked() (*Handle, error) { return newPrivateProcMount(false) }

func doMount(t *testing.T, source, target, fsType string, flags uintptr) {
	var sourcePath string
	if source != "" {
		// In order to be able to bind-mount a symlink source we need to
		// bind-mount using an O_PATH|O_NOFOLLOW of the source.
		file, err := os.OpenFile(source, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		require.NoError(t, err)
		defer runtime.KeepAlive(file)
		defer file.Close() //nolint:errcheck // test code
		sourcePath = fmt.Sprintf("/proc/self/fd/%d", file.Fd())
	}

	var targetPath string
	if target != "" {
		// In order to be able to mount on top of symlinks we need to
		// bind-mount through an O_PATH|O_NOFOLLOW of the target.
		file, err := os.OpenFile(target, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		require.NoError(t, err)
		defer runtime.KeepAlive(file)
		defer file.Close() //nolint:errcheck // test code
		targetPath = fmt.Sprintf("/proc/self/fd/%d", file.Fd())
	}

	err := unix.Mount(sourcePath, targetPath, fsType, flags, "")
	if errors.Is(err, unix.ENOENT) {
		// Future kernels will block these kinds of mounts by marking all of
		// these dentries with dont_mount(), which returns -ENOENT from mount.
		// See <https://lore.kernel.org/all/20240806-work-procfs-v1-0-fb04e1d09f0c@kernel.org/>,
		// which should make it into Linux 6.12. So ignore those errors.
		t.Skipf("current kernel does not allow /proc overmounts -- all proc operations are implicitly safe")
	}
	require.NoErrorf(t, err, "mount(%s<%s>, %s<%s>, %s, 0x%x)", sourcePath, source, targetPath, target, fsType, flags)
}

func setupMountNamespace(t *testing.T) {
	testutils.RequireRoot(t)

	// Lock our thread because we need to create a custom mount namespace. Each
	// test run is run in its own goroutine (this is not _explicitly_
	// guaranteed by Go but t.FailNow() uses Goexit, which means it has to be
	// true in practice) so locking the test to this thread means the other
	// tests will run on different goroutines.
	//
	// There is no UnlockOSThread() here, to ensure that the Go runtime will
	// kill this thread once this goroutine returns (ensuring no other
	// goroutines run in this context).
	runtime.LockOSThread()

	// New mount namespace (we are multi-threaded with a shared fs so we need
	// CLONE_FS to split us from the other threads in the Go process).
	err := unix.Unshare(unix.CLONE_FS | unix.CLONE_NEWNS)
	require.NoError(t, err, "new mount namespace")

	// Private /.
	err = unix.Mount("", "/", "", unix.MS_PRIVATE|unix.MS_REC, "")
	require.NoError(t, err)
}

func testProcThreadSelf(t *testing.T, procRoot *Handle, subpath string, expectErr bool) {
	handle, closer, err := procRoot.OpenThreadSelf(subpath)
	if expectErr {
		assert.ErrorIsf(t, err, errUnsafeProcfs, "should have detected /proc/thread-self/%s overmount", subpath)
	} else if assert.NoErrorf(t, err, "/proc/thread-self/%s open should succeed", subpath) {
		_ = handle.Close()
		closer() // LockOSThread stacks, so we can call this safely.
	}
}

type procRootFunc func() (*Handle, error)

func testProcOvermountSubdir(t *testing.T, procRootFn procRootFunc, expectOvermounts bool) {
	testForceProcThreadSelf(t, func(t *testing.T) {
		setupMountNamespace(t)

		// Create some overmounts on /proc/{thread-self/,self/}.
		for _, procThreadSelfPath := range []string{
			fmt.Sprintf("/proc/self/task/%d", unix.Gettid()),
			"/proc/self",
		} {
			for _, mount := range []struct {
				source, targetSubPath, fsType string
				flags                         uintptr
			}{
				// A tmpfs on top of /proc/thread-self/fdinfo to check whether
				// verifyProcRoot() works on old kernels.
				{"", "fdinfo", "tmpfs", 0},
				// A bind-mount of noop-write real procfs file on top of
				// /proc/thread-self/attr/current so we can test whether
				// verifyProcRoot() works for the file case.
				//
				// We don't use procThreadSelf for files in filepath-securejoin, but
				// this is to test the runc-equivalent behaviour for when this logic is
				// moved to libpathrs.
				{"/proc/self/sched", "attr/current", "", unix.MS_BIND},
				// Bind-mounts on top of symlinks should be detected by
				// checkSubpathOvermount.
				{"/proc/1/fd/0", "exe", "", unix.MS_BIND},
				{"/proc/1/exe", "fd/0", "", unix.MS_BIND},
				// TODO: Add a test for mounting on top of /proc/self or
				//       /proc/thread-self. This should be detected with openat2.
			} {
				target := path.Join(procThreadSelfPath, mount.targetSubPath)
				doMount(t, mount.source, target, mount.fsType, mount.flags)
			}
		}

		procRoot, err := procRootFn()
		require.NoError(t, err)
		defer procRoot.Close() //nolint:errcheck // test code

		// For both tmpfs and procfs overmounts, we should catch them (with or
		// without openat2, thanks to procfsLookupInRoot).
		testProcThreadSelf(t, procRoot, "fdinfo", expectOvermounts)
		testProcThreadSelf(t, procRoot, "attr/current", expectOvermounts)

		// For magic-links we expect to detect overmounts if there are any.
		symlinkOvermountErr := errUnsafeProcfs
		if !expectOvermounts {
			symlinkOvermountErr = nil
		}

		procSelf, closer, err := procRoot.OpenThreadSelf(".")
		require.NoError(t, err)
		defer procSelf.Close() //nolint:errcheck // test code
		defer closer()

		// Open these paths directly to emulate a non-openat2 handle that
		// didn't detect a bind-mount to check that checkSubpathOvermount works
		// properly for AT_EMPTY_PATH checks as well.
		procCwd, err := fd.Openat(procSelf, "cwd", unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		require.NoError(t, err)
		defer procCwd.Close() //nolint:errcheck // test code
		procExe, err := fd.Openat(procSelf, "exe", unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		require.NoError(t, err)
		defer procExe.Close() //nolint:errcheck // test code

		// no overmount
		err = checkSubpathOvermount(procRoot.Inner, procCwd, "")
		assert.NoError(t, err, "checking /proc/self/cwd with no overmount should succeed") //nolint:testifylint // this is an isolated operation so we can continue despite an error
		err = checkSubpathOvermount(procRoot.Inner, procSelf, "cwd")
		assert.NoError(t, err, "checking /proc/self/cwd with no overmount should succeed") //nolint:testifylint // this is an isolated operation so we can continue despite an error
		// basic overmount
		err = checkSubpathOvermount(procRoot.Inner, procExe, "")
		assert.ErrorIs(t, err, symlinkOvermountErr, "unexpected /proc/self/exe overmount result") //nolint:testifylint // this is an isolated operation so we can continue despite an error
		err = checkSubpathOvermount(procRoot.Inner, procSelf, "exe")
		assert.ErrorIs(t, err, symlinkOvermountErr, "unexpected /proc/self/exe overmount result") //nolint:testifylint // this is an isolated operation so we can continue despite an error

		// fd no overmount
		_, err = procRoot.Readlink(ProcThreadSelf, "fd/1")
		assert.NoError(t, err, "checking /proc/self/fd/1 with no overmount should succeed") //nolint:testifylint // this is an isolated operation so we can continue despite an error
		// fd overmount
		link, err := procRoot.Readlink(ProcThreadSelf, "fd/0")
		assert.ErrorIs(t, err, symlinkOvermountErr, "unexpected /proc/self/fd/0 overmount result: got link %q", link) //nolint:testifylint // this is an isolated operation so we can continue despite an error
	})
}

func tRunWrapper(t *testing.T) testutils.TRunFunc {
	return func(name string, doFn testutils.TDoFunc) {
		t.Run(name, func(t *testing.T) {
			doFn(t)
		})
	}
}

func TestProcOvermountSubdir_unsafeHostProcRoot(t *testing.T) {
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		// If we use the host /proc directly, we should see overmounts.
		testProcOvermountSubdir(t, unsafeHostProcRoot, true)
	})
}

func TestProcOvermountSubdir_newPrivateProcMountSubset(t *testing.T) {
	if !linux.HasNewMountAPI() {
		t.Skip("test requires fsopen/open_tree support")
	}
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		// If we create our own procfs, the overmounts shouldn't appear.
		testProcOvermountSubdir(t, newPrivateProcMountSubset, false)
	})
}

func TestProcOvermountSubdir_newPrivateProcMountUnmasked(t *testing.T) {
	if !linux.HasNewMountAPI() {
		t.Skip("test requires fsopen/open_tree support")
	}
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		// If we create our own procfs, the overmounts shouldn't appear.
		testProcOvermountSubdir(t, newPrivateProcMountUnmasked, false)
	})
}

func TestProcOvermountSubdir_clonePrivateProcMount(t *testing.T) {
	if !linux.HasNewMountAPI() {
		t.Skip("test requires fsopen/open_tree support")
	}
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		// If we use open_tree(2), we don't use AT_RECURSIVE when running in
		// this test (because the overmounts are not locked mounts) and so we
		// don't expect to see overmounts.
		testProcOvermountSubdir(t, clonePrivateProcMount, false)
	})
}

func TestProcOvermountSubdir_OpenProcRoot(t *testing.T) {
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		// We expect to not get overmounts if we have the new mount API.
		// FIXME: It's possible to hit overmounts if there are locked mounts
		// and we hit the AT_RECURSIVE case...
		procRootFn := func() (*Handle, error) { return getProcRoot(true) }
		testProcOvermountSubdir(t, procRootFn, !linux.HasNewMountAPI())
	})
}

func TestProcOvermountSubdir_OpenUnsafeProcRoot(t *testing.T) {
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		// We expect to not get overmounts if we have the new mount API.
		// FIXME: It's possible to hit overmounts if there are locked mounts
		// and we hit the AT_RECURSIVE case...
		testProcOvermountSubdir(t, OpenUnsafeProcRoot, !linux.HasNewMountAPI())
	})
}

func TestProcOvermountSubdir_getProcRootSubset_Mocked(t *testing.T) {
	if !linux.HasNewMountAPI() {
		t.Skip("test requires fsopen/open_tree support")
	}
	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		testForceGetProcRoot(t, func(t *testing.T, expectOvermounts bool) {
			procRootFn := func() (*Handle, error) { return getProcRoot(true) }
			testProcOvermountSubdir(t, procRootFn, expectOvermounts)
		})
	})
}

// isFsopenRoot returns whether the internal procfs handle is an fsopen root.
func isFsopenRoot(t *testing.T) bool {
	procRoot, err := OpenUnsafeProcRoot() // !subset=pid
	require.NoError(t, err)
	return procRoot.Inner.Name() == "fsmount:fscontext:proc"
}

// Because of the introduction of protections against /proc overmounts,
// ProcThreadSelf will not be called in actual tests unless we have a basic
// test here.
func TestProcThreadSelf(t *testing.T) {
	proc, err := OpenProcRoot()
	require.NoError(t, err)

	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		t.Run("stat", func(t *testing.T) {
			handle, closer, err := proc.OpenThreadSelf("stat")
			require.NoError(t, err, "ProcThreadSelf(stat)")
			require.NotNil(t, handle, "ProcThreadSelf(stat) handle")
			require.NotNil(t, closer, "ProcThreadSelf(stat) closer")
			defer closer()
			defer handle.Close() //nolint:errcheck // test code

			realPath, err := ProcSelfFdReadlink(handle)
			require.NoError(t, err)
			wantPath := fmt.Sprintf("/%d/task/%d/stat", os.Getpid(), unix.Gettid())
			if !isFsopenRoot(t) {
				// The /proc prefix is only present when not using fsopen.
				wantPath = "/proc" + wantPath
			}
			assert.Equal(t, wantPath, realPath, "final handle path")
		})

		t.Run("abspath", func(t *testing.T) {
			handle, closer, err := proc.OpenThreadSelf("/stat")
			require.NoError(t, err, "ProcThreadSelf(/stat)")
			require.NotNil(t, handle, "ProcThreadSelf(/stat) handle")
			require.NotNil(t, closer, "ProcThreadSelf(/stat) closer")
			defer closer()
			defer handle.Close() //nolint:errcheck // test code

			realPath, err := ProcSelfFdReadlink(handle)
			require.NoError(t, err)
			wantPath := fmt.Sprintf("/%d/task/%d/stat", os.Getpid(), unix.Gettid())
			if !isFsopenRoot(t) {
				// The /proc prefix is only present when not using fsopen.
				wantPath = "/proc" + wantPath
			}
			assert.Equal(t, wantPath, realPath, "final handle path")
		})

		t.Run("wacky-abspath", func(t *testing.T) {
			handle, closer, err := proc.OpenThreadSelf("////./////stat")
			require.NoError(t, err, "ProcThreadSelf(////./////stat)")
			require.NotNil(t, handle, "ProcThreadSelf(////./////stat) handle")
			require.NotNil(t, closer, "ProcThreadSelf(////./////stat) closer")
			defer closer()
			defer handle.Close() //nolint:errcheck // test code

			realPath, err := ProcSelfFdReadlink(handle)
			require.NoError(t, err)
			wantPath := fmt.Sprintf("/%d/task/%d/stat", os.Getpid(), unix.Gettid())
			if !isFsopenRoot(t) {
				// The /proc prefix is only present when not using fsopen.
				wantPath = "/proc" + wantPath
			}
			assert.Equal(t, wantPath, realPath, "final handle path")
		})

		t.Run("dotdot", func(t *testing.T) {
			handle, closer, err := proc.OpenThreadSelf("../../../../../../../../..")
			require.Error(t, err, "ProcThreadSelf(../...)")
			require.Nil(t, handle, "ProcThreadSelf(../...) handle")
			require.Nil(t, closer, "ProcThreadSelf(../...) closer")
		})

		t.Run("wacky-dotdot", func(t *testing.T) {
			handle, closer, err := proc.OpenThreadSelf("/../../../../../../../../..")
			require.Error(t, err, "ProcThreadSelf(/../...)")
			require.Nil(t, handle, "ProcThreadSelf(/../...) handle")
			require.Nil(t, closer, "ProcThreadSelf(/../...) closer")
		})
	})
}

func TestProcSelf(t *testing.T) {
	proc, err := OpenProcRoot()
	require.NoError(t, err)

	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		t.Run("stat", func(t *testing.T) {
			handle, err := proc.OpenSelf("stat")
			require.NoError(t, err, "ProcSelf(stat)")
			require.NotNil(t, handle, "ProcSelf(stat) handle")
			defer handle.Close() //nolint:errcheck // test code

			realPath, err := ProcSelfFdReadlink(handle)
			require.NoError(t, err)
			wantPath := fmt.Sprintf("/%d/stat", os.Getpid())
			if !isFsopenRoot(t) {
				// The /proc prefix is only present when not using fsopen.
				wantPath = "/proc" + wantPath
			}
			assert.Equal(t, wantPath, realPath, "final handle path")
		})

		t.Run("abspath", func(t *testing.T) {
			handle, err := proc.OpenSelf("/stat")
			require.NoError(t, err, "ProcSelf(/stat)")
			require.NotNil(t, handle, "ProcSelf(/stat) handle")
			defer handle.Close() //nolint:errcheck // test code

			realPath, err := ProcSelfFdReadlink(handle)
			require.NoError(t, err)
			wantPath := fmt.Sprintf("/%d/stat", os.Getpid())
			if !isFsopenRoot(t) {
				// The /proc prefix is only present when not using fsopen.
				wantPath = "/proc" + wantPath
			}
			assert.Equal(t, wantPath, realPath, "final handle path")
		})

		t.Run("wacky-abspath", func(t *testing.T) {
			handle, err := proc.OpenSelf("////./////stat")
			require.NoError(t, err, "ProcSelf(////./////stat)")
			require.NotNil(t, handle, "ProcSelf(////./////stat) handle")
			defer handle.Close() //nolint:errcheck // test code

			realPath, err := ProcSelfFdReadlink(handle)
			require.NoError(t, err)
			wantPath := fmt.Sprintf("/%d/stat", os.Getpid())
			if !isFsopenRoot(t) {
				// The /proc prefix is only present when not using fsopen.
				wantPath = "/proc" + wantPath
			}
			assert.Equal(t, wantPath, realPath, "final handle path")
		})

		t.Run("dotdot", func(t *testing.T) {
			handle, err := proc.OpenSelf("../../../../../../../../..")
			require.Error(t, err, "ProcSelf(../...)")
			require.Nil(t, handle, "ProcSelf(../...) handle")
		})

		t.Run("wacky-dotdot", func(t *testing.T) {
			handle, err := proc.OpenSelf("/../../../../../../../../..")
			require.Error(t, err, "ProcSelf(/../...)")
			require.Nil(t, handle, "ProcSelf(/../...) handle")
		})
	})
}

func TestProcPid(t *testing.T) {
	proc, err := OpenProcRoot()
	require.NoError(t, err)

	testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
		t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
		t.Run("pid1-stat", func(t *testing.T) {
			handle, err := proc.OpenPid(1, "stat")
			require.NoError(t, err, "ProcPid(1, stat)")
			require.NotNil(t, handle, "ProcPid(1, stat) handle")

			realPath, err := ProcSelfFdReadlink(handle)
			require.NoError(t, err)
			wantPath := "/1/stat"
			if !isFsopenRoot(t) {
				// The /proc prefix is only present when not using fsopen.
				wantPath = "/proc" + wantPath
			}
			assert.Equal(t, wantPath, realPath, "final handle path")
		})

		t.Run("pid1-stat-abspath", func(t *testing.T) {
			handle, err := proc.OpenPid(1, "/stat")
			require.NoError(t, err, "ProcPid(1, /stat)")
			require.NotNil(t, handle, "ProcPid(1, /stat) handle")

			realPath, err := ProcSelfFdReadlink(handle)
			require.NoError(t, err)
			wantPath := "/1/stat"
			if !isFsopenRoot(t) {
				// The /proc prefix is only present when not using fsopen.
				wantPath = "/proc" + wantPath
			}
			assert.Equal(t, wantPath, realPath, "final handle path")
		})

		t.Run("pid1-stat-wacky-abspath", func(t *testing.T) {
			handle, err := proc.OpenPid(1, "////.////stat")
			require.NoError(t, err, "ProcPid(1, ////.////stat)")
			require.NotNil(t, handle, "ProcPid(1, ////.////stat) handle")

			realPath, err := ProcSelfFdReadlink(handle)
			require.NoError(t, err)
			wantPath := "/1/stat"
			if !isFsopenRoot(t) {
				// The /proc prefix is only present when not using fsopen.
				wantPath = "/proc" + wantPath
			}
			assert.Equal(t, wantPath, realPath, "final handle path")
		})

		t.Run("dotdot", func(t *testing.T) {
			handle, err := proc.OpenPid(1, "../../../../../../../../..")
			require.Error(t, err, "ProcPid(1, ../...)")
			require.Nil(t, handle, "ProcPid(1, ../...) handle")
		})

		t.Run("wacky-dotdot", func(t *testing.T) {
			handle, err := proc.OpenPid(1, "/../../../../../../../../..")
			require.Error(t, err, "ProcPid(1, /../...)")
			require.Nil(t, handle, "ProcPid(1, /../...) handle")
		})
	})
}

func TestProcRoot(t *testing.T) {
	for _, test := range []struct {
		name       string
		procRootFn procRootFunc
	}{
		{"OpenProcRoot", OpenProcRoot},
		{"OpenUnsafeProcRoot", OpenUnsafeProcRoot},
	} {
		test := test // copy iterator
		t.Run(test.name, func(t *testing.T) {
			proc, err := test.procRootFn()
			require.NoError(t, err)

			testutils.WithWithoutOpenat2(true, tRunWrapper(t), func(ti testutils.TestingT) {
				t := ti.(*testing.T) //nolint:forcetypeassert // guaranteed to be true and in test code
				t.Run("sysctl", func(t *testing.T) {
					handle, err := proc.OpenRoot("sys/kernel/version")
					require.NoError(t, err, "ProcRoot(sys/kernel/version)")
					require.NotNil(t, handle, "ProcPid(sys/kernel/version) handle")

					realPath, err := ProcSelfFdReadlink(handle)
					require.NoError(t, err)
					wantPath := "/sys/kernel/version"
					if !isFsopenRoot(t) {
						// The /proc prefix is only present when not using fsopen.
						wantPath = "/proc" + wantPath
					}
					assert.Equal(t, wantPath, realPath, "final handle path")
				})
			})
		})
	}
}

func canFsOpen() bool {
	f, err := fd.Fsopen("tmpfs", 0)
	if f != nil {
		_ = f.Close()
	}
	return err == nil
}

func testProcOvermount(t *testing.T, procRootFn procRootFunc, privateProcMount bool) {
	testForceProcThreadSelf(t, func(t *testing.T) {
		for _, mount := range []struct {
			source, fsType string
			flags          uintptr
		}{
			// Try a non-procfs filesystem overmount.
			{"", "tmpfs", 0},
			// Try a procfs subdir overmount.
			{"/proc/tty", "bind", unix.MS_BIND},
		} {
			mount := mount // copy iterator
			t.Run("procmount="+mount.fsType, func(t *testing.T) {
				setupMountNamespace(t)
				doMount(t, mount.source, "/proc", mount.fsType, mount.flags)

				procRoot, err := procRootFn()
				if procRoot != nil {
					defer procRoot.Close() //nolint:errcheck // test code
				}
				if privateProcMount {
					assert.NoError(t, err, "get proc handle should succeed")                                      //nolint:testifylint
					assert.NoError(t, verifyProcRoot(procRoot.Inner), "verify private proc mount should succeed") //nolint:testifylint
				} else {
					if !assert.ErrorIs(t, err, errUnsafeProcfs, "get proc handle should fail") { //nolint:testifylint
						t.Logf("procRootFn() = %v, %v", procRoot, err)
					}
				}
			})
		}
	})
}

func TestProcOvermount_unsafeHostProcRoot(t *testing.T) {
	testProcOvermount(t, unsafeHostProcRoot, false)
}

func TestProcOvermount_clonePrivateProcMount(t *testing.T) {
	if !linux.HasNewMountAPI() {
		t.Skip("test requires open_tree support")
	}
	testProcOvermount(t, clonePrivateProcMount, false)
}

func TestProcOvermount_newPrivateProcMountSubset(t *testing.T) {
	if !linux.HasNewMountAPI() || !canFsOpen() {
		t.Skip("test requires fsopen support")
	}
	testProcOvermount(t, newPrivateProcMountSubset, true)
}

func TestProcOvermount_newPrivateProcMountUnmasked(t *testing.T) {
	if !linux.HasNewMountAPI() || !canFsOpen() {
		t.Skip("test requires fsopen support")
	}
	testProcOvermount(t, newPrivateProcMountUnmasked, true)
}

func TestProcOvermount_OpenProcRoot(t *testing.T) {
	privateProcMount := canFsOpen() && !testingForcePrivateProcRootOpenTree(nil)
	procRootFn := func() (*Handle, error) { return getProcRoot(true) }
	testProcOvermount(t, procRootFn, privateProcMount)
}

func TestProcOvermount_OpenProcRoot_Mocked(t *testing.T) {
	if !linux.HasNewMountAPI() {
		t.Skip("test requires fsopen/open_tree support")
	}
	testForceGetProcRoot(t, func(t *testing.T, _ bool) {
		privateProcMount := canFsOpen() && !testingForcePrivateProcRootOpenTree(nil)
		procRootFn := func() (*Handle, error) { return getProcRoot(true) }
		testProcOvermount(t, procRootFn, privateProcMount)
	})
}

func TestProcSelfFdPath(t *testing.T) {
	testForceProcThreadSelf(t, func(t *testing.T) {
		root := t.TempDir()

		filePath := path.Join(root, "file")
		err := unix.Mknod(filePath, unix.S_IFREG|0o644, 0)
		require.NoError(t, err)

		symPath := path.Join(root, "sym")
		err = unix.Symlink(filePath, symPath)
		require.NoError(t, err)

		// Open through the symlink.
		handle, err := os.Open(symPath)
		require.NoError(t, err)
		defer handle.Close() //nolint:errcheck // test code

		// The check should fail if we expect the symlink path.
		err = CheckProcSelfFdPath(symPath, handle)
		assert.ErrorIs(t, err, internal.ErrPossibleBreakout, "CheckProcSelfFdPath should fail for wrong path") //nolint:testifylint // this is an isolated operation so we can continue despite an error

		// The check should fail if we expect the symlink path.
		err = CheckProcSelfFdPath(filePath, handle)
		assert.NoError(t, err) //nolint:testifylint // this is an isolated operation so we can continue despite an error
	})
}

func TestProcSelfFdPath_DeadFile(t *testing.T) {
	testForceProcThreadSelf(t, func(t *testing.T) {
		root := t.TempDir()

		fullPath := path.Join(root, "file")
		handle, err := os.Create(fullPath)
		require.NoError(t, err)
		defer handle.Close() //nolint:errcheck // test code

		// The path still exists.
		err = CheckProcSelfFdPath(fullPath, handle)
		assert.NoError(t, err, "CheckProcSelfFdPath should succeed with regular file") //nolint:testifylint // this is an isolated operation so we can continue despite an error

		// Delete the path.
		err = os.Remove(fullPath)
		require.NoError(t, err)

		// The check should fail now.
		err = CheckProcSelfFdPath(fullPath, handle)
		assert.ErrorIs(t, err, internal.ErrDeletedInode, "CheckProcSelfFdPath should fail after deletion") //nolint:testifylint // this is an isolated operation so we can continue despite an error

		// The check should fail even if the expected path ends with " (deleted)".
		err = CheckProcSelfFdPath(fullPath+" (deleted)", handle)
		assert.ErrorIs(t, err, internal.ErrDeletedInode, "CheckProcSelfFdPath should fail after deletion even with (deleted) suffix") //nolint:testifylint // this is an isolated operation so we can continue despite an error
	})
}

func TestProcSelfFdPath_DeadDir(t *testing.T) {
	testForceProcThreadSelf(t, func(t *testing.T) {
		root := t.TempDir()

		fullPath := path.Join(root, "dir")
		err := os.Mkdir(fullPath, 0o755)
		require.NoError(t, err)

		handle, err := os.OpenFile(fullPath, unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		require.NoError(t, err)
		defer handle.Close() //nolint:errcheck // test code

		// The path still exists.
		err = CheckProcSelfFdPath(fullPath, handle)
		assert.NoError(t, err, "CheckProcSelfFdPath should succeed with regular directory") //nolint:testifylint // this is an isolated operation so we can continue despite an error

		// Delete the path.
		err = os.Remove(fullPath)
		require.NoError(t, err)

		// The check should fail now.
		err = CheckProcSelfFdPath(fullPath, handle)
		assert.ErrorIs(t, err, internal.ErrInvalidDirectory, "CheckProcSelfFdPath should fail after deletion") //nolint:testifylint // this is an isolated operation so we can continue despite an error

		// The check should fail even if the expected path ends with " (deleted)".
		err = CheckProcSelfFdPath(fullPath+" (deleted)", handle)
		assert.ErrorIs(t, err, internal.ErrInvalidDirectory, "CheckProcSelfFdPath should fail after deletion even with (deleted) suffix") //nolint:testifylint // this is an isolated operation so we can continue despite an error
	})
}

func testVerifyProcRoot(t *testing.T, procRoot string, expectedHandleErr, expectedRootErr error, errString string) {
	fakeProcRoot, err := os.OpenFile(procRoot, unix.O_PATH|unix.O_CLOEXEC, 0)
	require.NoError(t, err)
	defer fakeProcRoot.Close() //nolint:errcheck // test code

	err = verifyProcRoot(fakeProcRoot)
	require.ErrorIsf(t, err, expectedRootErr, "verifyProcRoot(%s)", procRoot)
	if expectedRootErr != nil {
		require.ErrorContainsf(t, err, errString, "verifyProcRoot(%s)", procRoot)
	}

	err = verifyProcHandle(fakeProcRoot)
	require.ErrorIsf(t, err, expectedHandleErr, "verifyProcHandle(%s)", procRoot)
	if expectedHandleErr != nil {
		require.ErrorContainsf(t, err, errString, "verifyProcHandle(%s)", procRoot)
	}
}

func TestVerifyProcRoot_Regular(t *testing.T) {
	testForceProcThreadSelf(t, func(t *testing.T) {
		testVerifyProcRoot(t, "/proc", nil, nil, "")
	})
}

func TestVerifyProcRoot_ProcNonRoot(t *testing.T) {
	testForceProcThreadSelf(t, func(t *testing.T) {
		testVerifyProcRoot(t, "/proc/self", nil, errUnsafeProcfs, "incorrect procfs root inode number")
		testVerifyProcRoot(t, "/proc/mounts", nil, errUnsafeProcfs, "incorrect procfs root inode number")
		testVerifyProcRoot(t, "/proc/stat", nil, errUnsafeProcfs, "incorrect procfs root inode number")
	})
}

func TestVerifyProcRoot_NotProc(t *testing.T) {
	testForceProcThreadSelf(t, func(t *testing.T) {
		testVerifyProcRoot(t, "/", errUnsafeProcfs, errUnsafeProcfs, "incorrect procfs root filesystem type")
		testVerifyProcRoot(t, ".", errUnsafeProcfs, errUnsafeProcfs, "incorrect procfs root filesystem type")
		testVerifyProcRoot(t, t.TempDir(), errUnsafeProcfs, errUnsafeProcfs, "incorrect procfs root filesystem type")
	})
}

func TestProcfsDummyHooks(t *testing.T) {
	assert.False(t, hookDummy(), "hookDummy should always return false")
	assert.False(t, hookDummyFile(nil), "hookDummyFile should always return false")
}

func TestCachedProcRoot_Close(t *testing.T) {
	proc := getCachedProcRoot()
	if proc == nil {
		t.Skip("cannot get proc handle")
	}

	f, err := proc.OpenSelf(".")
	require.NoError(t, err)
	_ = f.Close()

	for i := 0; i < 4; i++ {
		require.NoError(t, proc.Close(), "closing cached Handle")
	}

	f2, err := proc.OpenSelf(".")
	require.NoError(t, err)
	_ = f2.Close()
}
