package fullnode

import (
	"encoding/json"
	"io"
)

// TODO: can delete?
func mustWrite[T string | []byte](w io.Writer, s T) {
	_, err := io.WriteString(w, string(s))
	if err != nil {
		panic(err)
	}
}

// TODO: can delete?
func mustMarshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
