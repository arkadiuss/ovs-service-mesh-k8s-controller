/*
Copyright 2022.

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

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type NetworkStatus struct {
	Name string
	IPs  []string
}

var (
	consulAddr = "http://localhost:8500"
	VirtualIP  = "10.10.10.254"
)
var consulClient *consulapi.Client
var (
	RegisterAnnotation      = "ovs.servicemesh.arkadiuss.dev/consul-register"
	UpstreamsAnnotation     = "ovs.servicemesh.arkadiuss.dev/upstreams"
	NetworkaNameAnnotation  = "ovs.servicemesh.arkadiuss.dev/ovs-cni-network-name"
	NetworkStatusAnnotation = "k8s.v1.cni.cncf.io/network-status"
)

//+kubebuilder:rbac:groups=arkadiuss.dev,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=arkadiuss.dev,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=arkadiuss.dev,resources=pods/finalizers,verbs=update

// Inspired by: https://github.com/tczekajlo/kube-consul-register/blob/d710950a4ed16306787ad88516ab63ed3aa0ed8a/controller/pods/controller.go#L350
func PodContainerToConsulService(pod *corev1.Pod, containerStatus corev1.ContainerStatus) (*consulapi.AgentServiceRegistration, error) {
	service := &consulapi.AgentServiceRegistration{}

	service.Name = pod.Labels["app"]
	service.ID = fmt.Sprintf("%s-%s", pod.Name, containerStatus.Name)
	service.Tags = []string{"managed-by:ovs-servicemesh"}
	service.Port = GetContainerPort(pod, containerStatus.Name)

	var networks []NetworkStatus
	networksJsonString := pod.Annotations[NetworkStatusAnnotation]
	err := json.Unmarshal([]byte(networksJsonString), &networks)
	if err != nil {
		return nil, err
	}

	networkFound := false
	for _, networkStatus := range networks {
		if networkStatus.Name == pod.Annotations[NetworkaNameAnnotation] {
			networkFound = true
			service.Address = networkStatus.IPs[0]
		}
	}
	if !networkFound {
		return nil, errors.New("couldn't find right network")
	}

	upstreamValue, ok := pod.Annotations[UpstreamsAnnotation]
	if ok {
		var upstreams []consulapi.Upstream
		annotationUpstreams := strings.Split(upstreamValue, ",")

		for _, upstreamString := range annotationUpstreams {
			upstream := strings.Split(upstreamString, ":")
			var port int
			port, err = strconv.Atoi(upstream[1])
			if err != nil {
				return nil, err
			}

			upstreams = append(upstreams, consulapi.Upstream{
				LocalBindAddress: VirtualIP,
				LocalBindPort:    port,
				DestinationName:  upstream[0],
			})
		}
		service.Connect = &consulapi.AgentServiceConnect{
			SidecarService: &consulapi.AgentServiceRegistration{
				Proxy: &consulapi.AgentServiceConnectProxyConfig{
					Upstreams: upstreams,
				},
			},
		}
	}
	return service, nil
}

func GetContainerPort(pod *corev1.Pod, searchContainer string) int {
	for _, container := range pod.Spec.Containers {
		if container.Name == searchContainer {
			if len(container.Ports) > 0 {
				return int(container.Ports[0].ContainerPort)
			}
		}
	}
	return 0
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Pod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var log = logf.FromContext(ctx)
	log.Info("Reconciling", "pod", req.Name, "namespace", req.Namespace)
	pod := &corev1.Pod{}
	err := r.Client.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		log.Info("could not fetch Pod", "error", err.Error())
		return ctrl.Result{}, nil
	}

	consulClient, err = consulapi.NewClient(&consulapi.Config{
		Address: consulAddr,
	})
	if err != nil {
		log.Error(err, "unable to create consul client")
	}

	if isRegisteredStr, ok := pod.Annotations[RegisterAnnotation]; ok {
		isRegistered, err := strconv.ParseBool(isRegisteredStr)
		if err != nil || !isRegistered {
			return ctrl.Result{}, err
		}

		if pod.Status.Phase == v1.PodRunning {
			for _, container := range pod.Status.ContainerStatuses {

				if !container.Ready {
					continue
				}

				service, err := PodContainerToConsulService(pod, container)
				if err != nil {
					log.Error(err, "Can't convert POD to Consul's service")
					continue
				}

				log.Info("Container eligible for registration", "service", service)
				err = consulClient.Agent().ServiceRegister(service)
				if err != nil {
					log.Error(err, "Unable to register in consul")
				}
			}
		}

	} else {

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
