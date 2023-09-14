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
	Dir       string `json:"dir"`
	AllBytes  uint64 `json:"all_bytes,omitempty"`
	FreeBytes uint64 `json:"free_bytes,omitempty"`
	Error     string `json:"error,omitempty"`
}

// DiskUsage returns a handler which responds with disk statistics in JSON.
// Path is the filesystem path from which to check disk usage.
func DiskUsage(w http.ResponseWriter, r *http.Request) {
	var resp DiskUsageResponse
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		w.WriteHeader(http.StatusBadRequest)
		resp.Error = "query param dir must be specified"
		mustJSONEncode(resp, w)
		return
	}
	resp.Dir = dir
	var fs syscall.Statfs_t
	// Purposefully not adding test hook, so tests may catch OS issues.
	err := syscall.Statfs(filepath.Clean(dir), &fs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		resp.Error = err.Error()
		mustJSONEncode(resp, w)
		return
	}

	w.WriteHeader(http.StatusOK)
	var (
		all  = fs.Blocks * uint64(fs.Bsize)
		free = fs.Bfree * uint64(fs.Bsize)
	)
	resp.AllBytes = all
	resp.FreeBytes = free
	mustJSONEncode(resp, w)
}

func mustJSONEncode(v interface{}, w io.Writer) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}
}
