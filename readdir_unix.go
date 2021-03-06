// +build darwin freebsd linux netbsd openbsd

package godirwalk

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func readdirents(osDirname string, scratchBuffer []byte) (Dirents, error) {
	dh, err := os.Open(osDirname)
	if err != nil {
		return nil, err
	}
	fd := int(dh.Fd())

	if len(scratchBuffer) < MinimumScratchBufferSize {
		scratchBuffer = make([]byte, DefaultScratchBufferSize)
	}

	var entries Dirents
	var de *syscall.Dirent

	for {
		n, err := syscall.ReadDirent(fd, scratchBuffer)
		if err != nil {
			_ = dh.Close() // ignore potential error returned by Close
			return nil, err
		}
		if n <= 0 {
			break // end of directory reached
		}
		// Loop over the bytes returned by reading the directory entries.
		buf := scratchBuffer[:n]
		for len(buf) > 0 {
			de = (*syscall.Dirent)(unsafe.Pointer(&buf[0])) // point entry to first syscall.Dirent in buffer
			buf = buf[de.Reclen:]                           // advance buffer for next iteration through loop

			if inoFromDirent(de) == 0 {
				continue // this item has been deleted, but its entry not yet removed from directory listing
			}

			nameSlice := nameFromDirent(de)
			namlen := len(nameSlice)
			if (namlen == 0) || (namlen == 1 && nameSlice[0] == '.') || (namlen == 2 && nameSlice[0] == '.' && nameSlice[1] == '.') {
				continue // skip unimportant entries
			}
			osChildname := string(nameSlice)

			// Convert syscall constant, which is in purview of OS, to a
			// constant defined by Go, assumed by this project to be stable.
			var mode os.FileMode
			switch de.Type {
			case syscall.DT_REG:
				// regular file
			case syscall.DT_DIR:
				mode = os.ModeDir
			case syscall.DT_LNK:
				mode = os.ModeSymlink
			case syscall.DT_CHR:
				mode = os.ModeDevice | os.ModeCharDevice
			case syscall.DT_BLK:
				mode = os.ModeDevice
			case syscall.DT_FIFO:
				mode = os.ModeNamedPipe
			case syscall.DT_SOCK:
				mode = os.ModeSocket
			default:
				// If syscall returned unknown type (e.g., DT_UNKNOWN, DT_WHT),
				// then resolve actual mode by getting stat.
				fi, err := os.Lstat(filepath.Join(osDirname, osChildname))
				if err != nil {
					_ = dh.Close() // ignore potential error returned by Close
					return nil, err
				}
				// Even though the stat provided all file mode bits, we want to
				// ensure same values returned to caller regardless of whether
				// we obtained file mode bits from syscall or stat call.
				// Therefore mask out the additional file mode bits that are
				// provided by stat but not by the syscall, so users can rely on
				// their values.
				mode = fi.Mode() & os.ModeType
			}

			entries = append(entries, &Dirent{name: osChildname, modeType: mode})
		}
	}

	if err = dh.Close(); err != nil {
		return nil, err
	}
	return entries, nil
}

func readdirnames(osDirname string, scratchBuffer []byte) ([]string, error) {
	des, err := readdirents(osDirname, scratchBuffer)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(des))
	for i, v := range des {
		names[i] = v.name
	}
	return names, nil
}
