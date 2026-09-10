//go:build linux

package host

import "syscall"

func statfs(path string, output *syscallStatfs) error {
	var value syscall.Statfs_t
	if err := syscall.Statfs(path, &value); err != nil {
		return err
	}
	output.blocks = value.Blocks
	output.bfree = value.Bfree
	output.bavail = value.Bavail
	output.blockSize = uint64(value.Bsize)
	return nil
}
