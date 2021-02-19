/*
Copyright 2021 Guilhem Lettron.

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

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkv1alpha1 "github.com/guilhem/service-quorum-operator/api/v1alpha1"
)

// ServiceQuorumReconciler reconciles a ServiceQuorum object
type ServiceQuorumReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=network.barpilot.io,resources=servicesquorum,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network.barpilot.io,resources=servicesquorum/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=network.barpilot.io,resources=servicesquorum/finalizers,verbs=update

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;watch
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=replicasets/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=replicasets/status,verbs=get
//+kubebuilder:rbac:resources=services,verbs=get;list;watch;create;update;patch

func (r *ServiceQuorumReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("servicequorum", req.NamespacedName)

	var sq *networkv1alpha1.ServiceQuorum
	if err := r.Get(ctx, req.NamespacedName, sq); err != nil {
		log.Error(err, "unable to fetch ServiceQuorum")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var app client.Object

	if sq.Spec.Deployment != "" {
		var deploy *apps.Deployment
		if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: sq.Spec.Deployment}, deploy); err != nil {
			log.Error(err, "unable to fetch Deployment")
			// we'll ignore not-found errors, since they can't be fixed by an immediate
			// requeue (we'll need to wait for a new notification), and we can get them
			// on deleted requests.
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		app = deploy
	}

	var repList *apps.ReplicaSetList
	listOpt := client.ListOptions{Namespace: sq.GetNamespace()}
	if err := r.List(ctx, repList, &listOpt); err != nil {
		log.Error(err, "unable to fetch Deployment")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var Replicas []apps.ReplicaSet

	for _, i := range repList.Items {
		if i.Status.Replicas != 0 {
			owners := i.GetOwnerReferences()
			for _, o := range owners {
				if o.Kind == app.GetObjectKind().GroupVersionKind().Kind && o.Name == app.GetName() {
					Replicas = append(Replicas, i)
					sq.Status.ReplicaSets = append(sq.Status.ReplicaSets, i.Name)
					break
				}
			}
		}
	}

	var currentReplicaSet *apps.ReplicaSet
	for _, r := range Replicas {
		if currentReplicaSet.Status.AvailableReplicas < r.Status.AvailableReplicas {
			currentReplicaSet = &r
		}
	}
	sq.Status.CurrentReplicaSet = currentReplicaSet.GetName()

	// Service

	svc, err := constructServiceForServiceQuorum(sq, currentReplicaSet)
	if err != nil {
		return ctrl.Result{}, err
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.SetNamespace(svc.GetNamespace())
		if err := ctrl.SetControllerReference(sq, svc, r.Scheme); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	log.V(3).Info("Service %s %s", svc.GetName(), op)

	return ctrl.Result{}, nil
}

func constructServiceForServiceQuorum(sq *networkv1alpha1.ServiceQuorum, rs *apps.ReplicaSet) (*core.Service, error) {
	spec := sq.Spec.Template
	spec.Selector = rs.Spec.Selector.MatchLabels

	svc := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      sq.GetLabels(),
			Annotations: sq.GetAnnotations(),
			Name:        sq.GetName(),
			Namespace:   sq.GetNamespace(),
		},
		Spec: spec,
	}

	return svc, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceQuorumReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Trigger any serviceQuorum
	mapFn := handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
		serviceQuorumList := &networkv1alpha1.ServiceQuorumList{}

		if err := r.List(context.TODO(), serviceQuorumList); err != nil {
			return []reconcile.Request{}
		}

		reconcileRequests := make([]reconcile.Request, len(serviceQuorumList.Items))
		for _, sq := range serviceQuorumList.Items {
			reconcileRequests = append(reconcileRequests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      sq.GetName(),
					Namespace: sq.GetNamespace(),
				},
			})
		}
		return reconcileRequests
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.ServiceQuorum{}).
		Owns(&core.Service{}).
		Watches(&source.Kind{Type: &apps.ReplicaSet{}}, mapFn).
		Complete(r)
}
