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
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"net/http"
	"strings"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

const (
	admissionWebhookEnabledAnnotation = "sidecar-injector.atomix.io/enabled"
	admissionWebhookStatusAnnotation  = "sidecar-injector.atomix.io/status"
	admissionWebhookServiceAnnotation = "sidecar-injector.atomix.io/service"
	admissionWebhookClusterAnnotation = "sidecar-injector.atomix.io/cluster"
	admissionWebhookVersionAnnotation = "sidecar-injector.atomix.io/version"
	injected                          = "injected"
)

const (
	ConfigContainer = "atomix-config"
	AgentContainer  = "atomix-agent"
)

const (
	ConfigVolume = "atomix-config"
)

type WebhookServer struct {
	config string
	server *http.Server
}

type WebhookServerParameters struct {
	Port     int
	CertFile string
	KeyFile  string
	Config   string
}

func New(params WebhookServerParameters) *WebhookServer {
	config, err := loadConfig(params.Config)
	if err != nil {
		glog.Errorf("Filed to load configuration: %v", err)
	}

	pair, err := tls.LoadX509KeyPair(params.CertFile, params.KeyFile)
	if err != nil {
		glog.Errorf("Failed to load key pair: %v", err)
	}

	return &WebhookServer{
		config: config,
		server: &http.Server{
			Addr:      fmt.Sprintf(":%v", params.Port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}
}

func loadConfig(configFile string) (string, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return "", err
	}
	glog.Infof("New configuration: sha256sum %x", sha256.Sum256(data))
	return string(data), nil
}

type rfc6902PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1alpha1.AddToScheme(runtimeScheme)
}

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
}

func potentialPodName(metadata *metav1.ObjectMeta) string {
	if metadata.Name != "" {
		return metadata.Name
	}
	if metadata.GenerateName != "" {
		return metadata.GenerateName + "***** (actual name not yet known)"
	}
	return ""
}

func createPatch(pod *corev1.Pod, spec AgentSpec) []rfc6902PatchOperation {
	var patch []rfc6902PatchOperation

	patch = append(patch, addVolumes(pod.Spec.Volumes, newVolumes(spec), "/spec/volumes")...)
	patch = append(patch, addContainers(pod.Spec.InitContainers, newInitContainers(spec), "/spec/initContainers")...)
	patch = append(patch, addContainers(pod.Spec.Containers, newContainers(spec), "/spec/containers")...)
	patch = append(patch, addAnnotations(pod.Annotations, map[string]string{admissionWebhookStatusAnnotation: injected})...)

	return patch
}

func (wh *WebhookServer) inject(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v %s", err, string(req.Object.Raw))
		return toAdmissionResponse(err)
	}

	// Deal with potential empty fields, e.g., when the pod is created by a deployment
	podName := potentialPodName(&pod.ObjectMeta)
	if pod.ObjectMeta.Namespace == "" {
		pod.ObjectMeta.Namespace = req.Namespace
	}

	annotations := pod.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	enabled := annotations[admissionWebhookEnabledAnnotation]
	if strings.ToLower(enabled) != "true" {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	status := annotations[admissionWebhookStatusAnnotation]
	if strings.ToLower(status) == injected {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	cluster := annotations[admissionWebhookClusterAnnotation]
	service := annotations[admissionWebhookServiceAnnotation]
	if cluster == "" && service == "" {
		glog.Infof("Skipping %s/%s due to missing cluster annotation", pod.ObjectMeta.Namespace, podName)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	} else if service == "" {
		service = fmt.Sprintf("%s-service", cluster)
	}

	version := annotations[admissionWebhookVersionAnnotation]
	if version == "" {
		version = "latest"
	}

	glog.Infof("AdmissionReview for Kind=%v Namespace=%v Name=%v (%v) UID=%v Rfc6902PatchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, podName, req.UID, req.Operation, req.UserInfo)

	spec := AgentSpec{
		name:      podName,
		namespace: req.Namespace,
		config:    wh.config,
		service:   service,
		version:   version,
	}

	patch := createPatch(&pod, spec)
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		glog.Infof("AdmissionResponse: err=%v cluster=%v\n", err, cluster)
		return toAdmissionResponse(err)
	}

	glog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))

	reviewResponse := v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
	return &reviewResponse
}

func (wh *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Errorf("no body found")
		http.Error(w, "no body found", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("contentType=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, want `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var reviewResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Could not decode body: %v", err)
		reviewResponse = toAdmissionResponse(err)
	} else {
		reviewResponse = wh.inject(&ar)
	}

	response := v1beta1.AdmissionReview{}
	if reviewResponse != nil {
		response.Response = reviewResponse
		if ar.Request != nil {
			response.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(response)
	if err != nil {
		glog.Errorf("Could not encode response: %v", err)
		http.Error(w, fmt.Sprintf("could encode response: %v", err), http.StatusInternalServerError)
	}
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Could not write response: %v", err)
		http.Error(w, fmt.Sprintf("could write response: %v", err), http.StatusInternalServerError)
	}
}

// Starts the webhook server
func (wh *WebhookServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", wh.serve)
	wh.server.Handler = mux

	// start webhook server in new routine
	go func() {
		if err := wh.server.ListenAndServeTLS("", ""); err != nil {
			glog.Errorf("Failed to listen and serve webhook server: %v", err)
		}
	}()
}

func (wh *WebhookServer) Stop() {
	wh.server.Shutdown(context.Background())
}
