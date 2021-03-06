/*
 * Copyright 2019 Open Networking Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package webhook

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strings"
)

type AgentSpec struct {
	name      string
	namespace string
	config    string
	service   string
	version   string
}

func addContainers(target, added []corev1.Container, basePath string) (patch []rfc6902PatchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Container{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, rfc6902PatchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func newInitContainers(spec AgentSpec) []corev1.Container {
	return []corev1.Container{
		newConfigContainer(spec),
	}
}

func newConfigContainer(spec AgentSpec) corev1.Container {
	return corev1.Container{
		Name:  ConfigContainer,
		Image: "busybox",
		Command: []string{
			"sh",
			"-c",
			"echo \"" + newConfigOutput(spec) + "\" > /config/atomix.conf",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      ConfigVolume,
				MountPath: "/config",
			},
		},
	}
}

func newConfigOutput(spec AgentSpec) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("atomix.node.id=$(hostname -s)"))
	lines = append(lines, fmt.Sprintf("atomix.node.host=$(hostname -s)"))
	lines = append(lines, fmt.Sprintf("atomix.service=\"%s.%s.svc.cluster.local\"", spec.service, spec.namespace))
	lines = append(lines, strings.Replace(spec.config, "$", "\\$", -1))
	return strings.Join(lines, "\n")
}

func newContainers(spec AgentSpec) []corev1.Container {
	return []corev1.Container{
		newAgentContainer(spec),
	}
}

func newAgentContainer(spec AgentSpec) corev1.Container {
	return corev1.Container{
		Name:            AgentContainer,
		Image:           fmt.Sprintf("atomix/atomix:%s", spec.version),
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{
				Name:          "client",
				ContainerPort: 5678,
			},
			{
				Name:          "server",
				ContainerPort: 5679,
			},
		},
		Args: []string{
			"--config",
			"/etc/atomix/atomix.conf",
			"--log-level=INFO",
			"--file-log-level=OFF",
			"--console-log-level=INFO",
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/v1/status",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 5678},
				},
			},
			InitialDelaySeconds: 60,
			TimeoutSeconds:      10,
			FailureThreshold:    6,
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/v1/status",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 5678},
				},
			},
			InitialDelaySeconds: 60,
			TimeoutSeconds:      10,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      ConfigVolume,
				MountPath: "/etc/atomix",
			},
		},
	}
}

func addVolumes(target, added []corev1.Volume, basePath string) (patch []rfc6902PatchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Volume{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, rfc6902PatchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func newVolumes(spec AgentSpec) []corev1.Volume {
	return []corev1.Volume{
		newConfigVolume(),
	}
}

func newConfigVolume() corev1.Volume {
	return corev1.Volume{
		Name: ConfigVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func escapeJSONPointerValue(in string) string {
	step := strings.Replace(in, "~", "~0", -1)
	return strings.Replace(step, "/", "~1", -1)
}

func addAnnotations(target map[string]string, added map[string]string) (patch []rfc6902PatchOperation) {
	for key, value := range added {
		if target == nil {
			target = map[string]string{}
			patch = append(patch, rfc6902PatchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			op := "add"
			if target[key] != "" {
				op = "replace"
			}
			patch = append(patch, rfc6902PatchOperation{
				Op:    op,
				Path:  "/metadata/annotations/" + escapeJSONPointerValue(key),
				Value: value,
			})
		}
	}
	return patch
}
