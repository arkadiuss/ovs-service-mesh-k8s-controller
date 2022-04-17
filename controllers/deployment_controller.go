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
	"io"
	"net/http"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Deployment object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// _ = log.FromContext(ctx)
	var log = logf.FromContext(ctx)

	deployment := &appsv1.Deployment{}
	err := r.Client.Get(ctx, req.NamespacedName, deployment)
	// if errors.IsNotFound(err) {
	// 	log.Error(nil, "Could not find Deployment")
	// 	return reconcile.Result{}, nil
	// }

	if err != nil {
		log.Error(err, "could not fetch Deployment")
		return reconcile.Result{}, err
	}

	log.Info("Reconciling %s from namespace %s", req.Name, req.Namespace)

	log.Info("Looking for consul instance...")

	resp, err := http.Get("http://localhost:8500/v1/catalog/services")
	if err != nil {
		log.Error(err, "could not fetch consul")
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		log.Info("Got response from consul: %s", body)

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}
