package kube

import (
	"encoding/hex"
	"encoding/json"
	"hash/fnv"

	"golang.org/x/exp/constraints"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Adapt adapts a kubernetes client.Object into a diffable Resource which can be used by Diff.
// The revision is an FNV-1 hash of the object's JSON representation.
func Adapt[T client.Object, I constraints.Integer](obj T, ordinal I) Resource[T] {
	b, err := json.Marshal(obj)
	if err != nil {
		// If we can't marshal a kube object, something is very wrong.
		panic(err)
	}
	h := fnv.New32()
	_, err = h.Write(b)
	if err != nil {
		// Similarly, if this write fails, something is very wrong.
		panic(err)
	}
	rev := hex.EncodeToString(h.Sum(nil))
	return adapter[T, I]{obj: obj, ordinal: ordinal, revision: rev}
}

type adapter[T client.Object, I constraints.Integer] struct {
	obj      T
	ordinal  I
	revision string
}

func (a adapter[T, I]) Object() T        { return a.obj }
func (a adapter[T, I]) Revision() string { return a.revision }
func (a adapter[T, I]) Ordinal() int64   { return int64(a.ordinal) }
