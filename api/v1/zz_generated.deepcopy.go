//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2022 Strangelove Ventures LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosAppConfig) DeepCopyInto(out *CosmosAppConfig) {
	*out = *in
	if in.Pruning != nil {
		in, out := &in.Pruning, &out.Pruning
		*out = new(CosmosPruning)
		(*in).DeepCopyInto(*out)
	}
	if in.HaltHeight != nil {
		in, out := &in.HaltHeight, &out.HaltHeight
		*out = new(uint64)
		**out = **in
	}
	if in.TomlOverrides != nil {
		in, out := &in.TomlOverrides, &out.TomlOverrides
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosAppConfig.
func (in *CosmosAppConfig) DeepCopy() *CosmosAppConfig {
	if in == nil {
		return nil
	}
	out := new(CosmosAppConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosChainConfig) DeepCopyInto(out *CosmosChainConfig) {
	*out = *in
	in.Tendermint.DeepCopyInto(&out.Tendermint)
	in.App.DeepCopyInto(&out.App)
	if in.LogLevel != nil {
		in, out := &in.LogLevel, &out.LogLevel
		*out = new(string)
		**out = **in
	}
	if in.LogFormat != nil {
		in, out := &in.LogFormat, &out.LogFormat
		*out = new(string)
		**out = **in
	}
	if in.GenesisURL != nil {
		in, out := &in.GenesisURL, &out.GenesisURL
		*out = new(string)
		**out = **in
	}
	if in.GenesisScript != nil {
		in, out := &in.GenesisScript, &out.GenesisScript
		*out = new(string)
		**out = **in
	}
	if in.SnapshotURL != nil {
		in, out := &in.SnapshotURL, &out.SnapshotURL
		*out = new(string)
		**out = **in
	}
	if in.SnapshotScript != nil {
		in, out := &in.SnapshotScript, &out.SnapshotScript
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosChainConfig.
func (in *CosmosChainConfig) DeepCopy() *CosmosChainConfig {
	if in == nil {
		return nil
	}
	out := new(CosmosChainConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosFullNode) DeepCopyInto(out *CosmosFullNode) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosFullNode.
func (in *CosmosFullNode) DeepCopy() *CosmosFullNode {
	if in == nil {
		return nil
	}
	out := new(CosmosFullNode)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CosmosFullNode) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosFullNodeList) DeepCopyInto(out *CosmosFullNodeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CosmosFullNode, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosFullNodeList.
func (in *CosmosFullNodeList) DeepCopy() *CosmosFullNodeList {
	if in == nil {
		return nil
	}
	out := new(CosmosFullNodeList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CosmosFullNodeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosFullNodeSpec) DeepCopyInto(out *CosmosFullNodeSpec) {
	*out = *in
	in.ChainConfig.DeepCopyInto(&out.ChainConfig)
	in.PodTemplate.DeepCopyInto(&out.PodTemplate)
	in.RolloutStrategy.DeepCopyInto(&out.RolloutStrategy)
	in.VolumeClaimTemplate.DeepCopyInto(&out.VolumeClaimTemplate)
	in.RPCServiceTemplate.DeepCopyInto(&out.RPCServiceTemplate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosFullNodeSpec.
func (in *CosmosFullNodeSpec) DeepCopy() *CosmosFullNodeSpec {
	if in == nil {
		return nil
	}
	out := new(CosmosFullNodeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosFullNodeStatus) DeepCopyInto(out *CosmosFullNodeStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosFullNodeStatus.
func (in *CosmosFullNodeStatus) DeepCopy() *CosmosFullNodeStatus {
	if in == nil {
		return nil
	}
	out := new(CosmosFullNodeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosMetadata) DeepCopyInto(out *CosmosMetadata) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosMetadata.
func (in *CosmosMetadata) DeepCopy() *CosmosMetadata {
	if in == nil {
		return nil
	}
	out := new(CosmosMetadata)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosPersistentVolumeClaim) DeepCopyInto(out *CosmosPersistentVolumeClaim) {
	*out = *in
	in.Resources.DeepCopyInto(&out.Resources)
	if in.AccessModes != nil {
		in, out := &in.AccessModes, &out.AccessModes
		*out = make([]corev1.PersistentVolumeAccessMode, len(*in))
		copy(*out, *in)
	}
	if in.VolumeMode != nil {
		in, out := &in.VolumeMode, &out.VolumeMode
		*out = new(corev1.PersistentVolumeMode)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosPersistentVolumeClaim.
func (in *CosmosPersistentVolumeClaim) DeepCopy() *CosmosPersistentVolumeClaim {
	if in == nil {
		return nil
	}
	out := new(CosmosPersistentVolumeClaim)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosPodSpec) DeepCopyInto(out *CosmosPodSpec) {
	*out = *in
	in.Metadata.DeepCopyInto(&out.Metadata)
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Priority != nil {
		in, out := &in.Priority, &out.Priority
		*out = new(int32)
		**out = **in
	}
	in.Resources.DeepCopyInto(&out.Resources)
	if in.TerminationGracePeriodSeconds != nil {
		in, out := &in.TerminationGracePeriodSeconds, &out.TerminationGracePeriodSeconds
		*out = new(int64)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosPodSpec.
func (in *CosmosPodSpec) DeepCopy() *CosmosPodSpec {
	if in == nil {
		return nil
	}
	out := new(CosmosPodSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosPruning) DeepCopyInto(out *CosmosPruning) {
	*out = *in
	if in.Interval != nil {
		in, out := &in.Interval, &out.Interval
		*out = new(uint32)
		**out = **in
	}
	if in.KeepEvery != nil {
		in, out := &in.KeepEvery, &out.KeepEvery
		*out = new(uint32)
		**out = **in
	}
	if in.KeepRecent != nil {
		in, out := &in.KeepRecent, &out.KeepRecent
		*out = new(uint32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosPruning.
func (in *CosmosPruning) DeepCopy() *CosmosPruning {
	if in == nil {
		return nil
	}
	out := new(CosmosPruning)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosRPCServiceSpec) DeepCopyInto(out *CosmosRPCServiceSpec) {
	*out = *in
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Type != nil {
		in, out := &in.Type, &out.Type
		*out = new(corev1.ServiceType)
		**out = **in
	}
	if in.ExternalTrafficPolicy != nil {
		in, out := &in.ExternalTrafficPolicy, &out.ExternalTrafficPolicy
		*out = new(corev1.ServiceExternalTrafficPolicyType)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosRPCServiceSpec.
func (in *CosmosRPCServiceSpec) DeepCopy() *CosmosRPCServiceSpec {
	if in == nil {
		return nil
	}
	out := new(CosmosRPCServiceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosRolloutStrategy) DeepCopyInto(out *CosmosRolloutStrategy) {
	*out = *in
	if in.MaxUnavailable != nil {
		in, out := &in.MaxUnavailable, &out.MaxUnavailable
		*out = new(intstr.IntOrString)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosRolloutStrategy.
func (in *CosmosRolloutStrategy) DeepCopy() *CosmosRolloutStrategy {
	if in == nil {
		return nil
	}
	out := new(CosmosRolloutStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CosmosTendermintConfig) DeepCopyInto(out *CosmosTendermintConfig) {
	*out = *in
	if in.MaxInboundPeers != nil {
		in, out := &in.MaxInboundPeers, &out.MaxInboundPeers
		*out = new(int32)
		**out = **in
	}
	if in.MaxOutboundPeers != nil {
		in, out := &in.MaxOutboundPeers, &out.MaxOutboundPeers
		*out = new(int32)
		**out = **in
	}
	if in.CorsAllowedOrigins != nil {
		in, out := &in.CorsAllowedOrigins, &out.CorsAllowedOrigins
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.TomlOverrides != nil {
		in, out := &in.TomlOverrides, &out.TomlOverrides
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CosmosTendermintConfig.
func (in *CosmosTendermintConfig) DeepCopy() *CosmosTendermintConfig {
	if in == nil {
		return nil
	}
	out := new(CosmosTendermintConfig)
	in.DeepCopyInto(out)
	return out
}
