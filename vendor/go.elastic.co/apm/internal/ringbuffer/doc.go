// Package ringbuffer provides a ring buffer for storing blocks of bytes.
// Bytes are written and read in discrete blocks. If the buffer becomes
// full, then writing to it will evict the oldest blocks until there is
// space for a new one.
package ringbuffer
