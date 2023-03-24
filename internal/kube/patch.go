package kube

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

var converter = runtime.DefaultUnstructuredConverter

// ApplyStrategicPatch computes a strategic patch from original and patch, and applies the patch to the original.
// Inspired by: https://github.com/kubernetes/apiserver/blob/45f55ded302a02ed2023e8b45bd241cf7d81169e/pkg/endpoints/handlers/patch.go
func ApplyStrategicPatch[T any](original, patch T) error {
	origJSON, err := json.Marshal(original)
	if err != nil {
		return err
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	schema, err := strategicpatch.NewPatchMetaFromStruct(original)
	if err != nil {
		return err
	}

	applyPatch, err := strategicpatch.CreateTwoWayMergePatchUsingLookupPatchMeta(origJSON, patchJSON, schema)
	if err != nil {
		return err
	}

	fmt.Println("Patch:", string(applyPatch))

	result, err := strategicpatch.StrategicMergePatchUsingLookupPatchMeta(origJSON, applyPatch, schema)
	if err != nil {
		return err
	}
	fmt.Println("Orig:", string(origJSON))
	fmt.Println("Result:", string(result))

	//origMap, err := converter.ToUnstructured(original)
	//if err != nil {
	//	panic(err)
	//	return err
	//}
	//patchMap, err := converter.ToUnstructured(patch)
	//if err != nil {
	//	panic(err)
	//	return err
	//}
	//
	//schema, err := strategicpatch.NewPatchMetaFromStruct(original)
	//if err != nil {
	//	panic(err)
	//	return err
	//}
	//
	//applyPatch, err := strategicpatch.CreateTwoWayMergeMapPatchUsingLookupPatchMeta(origMap, patchMap, schema)
	//if err != nil {
	//	panic(err)
	//	return err
	//}
	//
	//result, err := strategicpatch.StrategicMergeMapPatch(origMap, applyPatch, original)
	//fmt.Println("Result:", result)
	//if err != nil {
	//	panic(err)
	//	return err
	//}
	//
	//fmt.Println("Original before:", original)
	//err = runtime.DefaultUnstructuredConverter.FromUnstructured(result, original)
	//fmt.Println("Original after:", original)
	//return err // TODO: useless return
	return nil
}
