package logs

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
	"time"
)

// ReadTail returns the last maxLines lines of path. If the file does not exist, returns empty slice.
func ReadTail(path string, maxLines int) ([]string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, 0, nil
		}
		return nil, 0, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}

	// Read at most the last 512KB for fast tailing
	const maxBytes = 512 * 1024
	size := info.Size()
	var offset int64
	if size > maxBytes {
		offset = size - maxBytes
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, 0, err
		}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lines := make([]string, 0, maxLines)
	firstPartial := offset > 0
	for scanner.Scan() {
		if firstPartial {
			firstPartial = false
			continue
		}
		lines = append(lines, strings.TrimRight(scanner.Text(), "\r"))
	}

	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, size, nil
}

// Clear truncates the log file in place so tail -f in the proxy container continues working.
func Clear(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return f.Close()
}

// Stream polls path for newly appended lines starting from startOffset and calls onLine for each line.
func Stream(ctx context.Context, path string, startOffset int64, onLine func(line string)) {
	offset := startOffset
	var partial string
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.Size() < offset {
				// File truncated or recreated
				offset = 0
				partial = ""
			}
			if info.Size() == offset {
				continue
			}
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			if _, err := f.Seek(offset, io.SeekStart); err == nil {
				data, err := io.ReadAll(f)
				if err == nil && len(data) > 0 {
					offset += int64(len(data))
					chunk := partial + string(data)
					parts := strings.Split(chunk, "\n")
					partial = parts[len(parts)-1]
					for _, line := range parts[:len(parts)-1] {
						onLine(strings.TrimRight(line, "\r"))
					}
				}
			}
			_ = f.Close()
		}
	}
}
