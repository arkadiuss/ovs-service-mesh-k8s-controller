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

	consul "arkadiuss.dev/ovs-service-mesh-controller/controllers/consul"
	ovs "arkadiuss.dev/ovs-service-mesh-controller/controllers/ovs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=arkadiuss.dev,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=arkadiuss.dev,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=arkadiuss.dev,resources=pods/finalizers,verbs=update

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
	// log.Info("Reconciling", "pod", req.Name, "namespace", req.Namespace)
	pod := &corev1.Pod{}
	err := r.Client.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		log.Info("could not fetch Pod", "error", err.Error())
		return ctrl.Result{}, nil
	}

	consulPod := &corev1.Pod{}
	consuPodName := types.NamespacedName{
		Namespace: "consul",
		Name:      "consul-server-0",
	}
	err = r.Client.Get(ctx, consuPodName, consulPod)
	if err != nil {
		log.Info("could not fetch Consul Pod", "error", err.Error())
		return ctrl.Result{}, nil
	}
	// log.Info("Consul is at: ", "address", consulPod.Status.PodIP, "address", consulPod.Status.HostIP)

	registeredServiceNames, err := consul.RegisterPodInConsul(pod, ctx)
	if err != nil {
		log.Error(err, "Unalbe to register pod in consul")
	}
	if registeredServiceNames != nil {
		for _, registeredService := range *registeredServiceNames {
			ovs.CreateOVSFlows(registeredService, ctx, r.Client)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
