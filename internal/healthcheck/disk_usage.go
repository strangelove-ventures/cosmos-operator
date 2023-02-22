package healthcheck

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"syscall"
)

// DiskUsageResponse returns disk statistics in bytes.
type DiskUsageResponse struct {
	AllBytes  uint64 `json:"all_bytes"`
	FreeBytes uint64 `json:"free_bytes"`
}

// DiskUsage returns a handler which responds with disk statistics in JSON.
// Path is the filesystem path from which to check disk usage.
func DiskUsage(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var fs syscall.Statfs_t
		// Purposefully not adding test hook, so tests may catch OS issues.
		err := syscall.Statfs(filepath.Clean(dir), &fs)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			mustJSONEncode(map[string]string{"error": err.Error()}, w)
			return
		}

		w.WriteHeader(http.StatusOK)
		var (
			all  = fs.Blocks * uint64(fs.Bsize)
			free = fs.Bfree * uint64(fs.Bsize)
		)
		mustJSONEncode(DiskUsageResponse{AllBytes: all, FreeBytes: free}, w)
	}
}

func mustJSONEncode(v interface{}, w io.Writer) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}
}
