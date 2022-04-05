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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/digihunch/wordpress-operator/api/v1"
	wordpressv1 "github.com/digihunch/wordpress-operator/api/v1"
)

// WordPressReconciler reconciles a WordPress object
type WordPressReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=wordpress.digihunch.com,resources=wordpresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=wordpress.digihunch.com,resources=wordpresses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=wordpress.digihunch.com,resources=wordpresses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WordPress object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *WordPressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("wordpress", req.NamespacedName)

	// User logic start from here
	r.Log.Info("Reconciling WordPress")

	wordpress := &v1.WordPress{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, wordpress)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	var result *ctrl.Result

	// === MYSQL ======

	result, err = r.ensurePVC(req, wordpress, r.pvcForMysql(wordpress))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureDeployment(req, wordpress, r.deploymentForMysql(wordpress))
	if result != nil {
		return *result, err
	}
	result, err = r.ensureService(req, wordpress, r.serviceForMysql(wordpress))
	if result != nil {
		return *result, err
	}

	mysqlRunning := r.isMysqlUp(wordpress)

	if !mysqlRunning {
		// If MySQL isn't running yet, requeue the reconcile
		// to run again after a delay
		delay := time.Second * time.Duration(5)

		r.Log.Info(fmt.Sprintf("MySQL isn't running, waiting for %s", delay))
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	// ===== WORDPRESS =====

	result, err = r.ensurePVC(req, wordpress, r.pvcForWordPress(wordpress))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureDeployment(req, wordpress, r.deploymentForWordPress(wordpress))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureService(req, wordpress, r.serviceForWordPress(wordpress))
	if result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WordPressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wordpressv1.WordPress{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
