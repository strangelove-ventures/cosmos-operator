package kube

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

var converter = runtime.DefaultUnstructuredConverter

// ApplyStrategicPatch applies a strategic merge patch to a target object.
// Inspired by: https://github.com/kubernetes/apiserver/blob/45f55ded302a02ed2023e8b45bd241cf7d81169e/pkg/endpoints/handlers/patch.go
func ApplyStrategicPatch[T any](target, patch T) error {
	targetMap, err := converter.ToUnstructured(target)
	if err != nil {
		return err
	}
	patchMap, err := converter.ToUnstructured(patch)
	if err != nil {
		return err
	}
	schema, err := strategicpatch.NewPatchMetaFromStruct(target)
	if err != nil {
		return err
	}
	result, err := strategicpatch.StrategicMergeMapPatchUsingLookupPatchMeta(targetMap, patchMap, schema)
	if err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(result, target)
}
