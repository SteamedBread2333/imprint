package plugin

import (
	"context"
	"io"
	"os"
	"time"
)

// followLogFile streams appended bytes from path to w until ctx is canceled.
func followLogFile(ctx context.Context, path string, w io.Writer) {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			offset = tailFromOffset(path, offset, w)
		}
	}
}

func tailFromOffset(path string, offset int64, w io.Writer) int64 {
	f, err := os.Open(path)
	if err != nil {
		return offset
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return offset
	}
	size := st.Size()
	if size < offset {
		offset = 0
	}
	if size <= offset {
		return offset
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset
	}
	n, err := io.Copy(w, f)
	if err != nil {
		return offset
	}
	return offset + n
}
