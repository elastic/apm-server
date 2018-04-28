package elasticapm

import (
	"bytes"
	"syscall"
	"unsafe"
)

func currentProcessTitle() (string, error) {
	// PR_GET_NAME (since Linux 2.6.11)
	// Return the name of the calling thread, in the buffer pointed to by
	// (char *) arg2.  The buffer should allow space for up to 16 bytes;
	// the returned string will be null-terminated.
	var buf [16]byte
	if _, _, errno := syscall.RawSyscall6(
		syscall.SYS_PRCTL, syscall.PR_GET_NAME,
		uintptr(unsafe.Pointer(&buf[0])),
		0, 0, 0, 0,
	); errno != 0 {
		return "", errno
	}
	return string(buf[:bytes.IndexByte(buf[:], 0)]), nil
}
