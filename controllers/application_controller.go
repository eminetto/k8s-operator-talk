/*
Copyright 2023.

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

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	minettodevv1alpha1 "github.com/eminetto/k8s-operator-talk/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const finalizer = "minetto.dev/application_controller_finalizer"

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=minetto.dev,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=minetto.dev,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=minetto.dev,resources=applications/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Application object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	var app minettodevv1alpha1.Application
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if apierrors.IsNotFound(err) {
			// we'll ignore not-found errors, since they can't be fixed by an immediate
			// requeue (we'll need to wait for a new notification), and we can get them
			// on deleted requests.
			return ctrl.Result{}, nil
		}
		l.Error(err, "unable to fetch Application")
		return ctrl.Result{}, err
	}
	if !controllerutil.ContainsFinalizer(&app, finalizer) {
		l.Info("Adding Finalizer")
		controllerutil.AddFinalizer(&app, finalizer)
		return ctrl.Result{}, r.Update(ctx, &app)
	}

	if !app.DeletionTimestamp.IsZero() {
		// invocar função que faz o delete
		l.Info("Application is being deleted")
		return r.reconcileDelete(ctx, &app)
	}
	// invocar função que faz o create
	l.Info("Application is being created")
	return r.reconcileCreate(ctx, &app)
}

func (r *ApplicationReconciler) reconcileCreate(ctx context.Context, app *minettodevv1alpha1.Application) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("Creating namespace")
	err := r.createNamespace(ctx, app)
	if err != nil {
		return ctrl.Result{}, err
	}
	l.Info("Creating deployment")
	err = r.createDeployment(ctx, app)
	if err != nil {
		return ctrl.Result{}, err
	}
	l.Info("Creating service")
	err = r.createService(ctx, app)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) createNamespace(ctx context.Context, app *minettodevv1alpha1.Application) error {
	ns := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: app.ObjectMeta.Name,
		},
	}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, &ns, func() error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to create Namespace: %v", err)
	}
	return nil
}

func (r *ApplicationReconciler) createDeployment(ctx context.Context, app *minettodevv1alpha1.Application) error {
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        app.ObjectMeta.Name + "-deployment",
			Namespace:   app.ObjectMeta.Name,
			Labels:      map[string]string{"label": app.ObjectMeta.Name, "app": app.ObjectMeta.Name},
			Annotations: map[string]string{"imageregistry": "https://hub.docker.com/"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &app.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"label": app.ObjectMeta.Name},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"label": app.ObjectMeta.Name, "app": app.ObjectMeta.Name},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  app.ObjectMeta.Name + "-container",
							Image: app.Spec.Image,
							Ports: []v1.ContainerPort{
								{
									ContainerPort: app.Spec.Port,
								},
							},
						},
					},
				},
			},
		},
	}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, &depl, func() error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to create Deployment: %v", err)
	}
	return nil
}

func (r *ApplicationReconciler) createService(ctx context.Context, app *minettodevv1alpha1.Application) error {
	srv := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.ObjectMeta.Name + "-service",
			Namespace: app.ObjectMeta.Name,
			Labels:    map[string]string{"app": app.ObjectMeta.Name},
		},
		Spec: v1.ServiceSpec{
			Type:                  v1.ServiceTypeNodePort,
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
			Selector:              map[string]string{"app": app.ObjectMeta.Name},
			Ports: []v1.ServicePort{
				{
					Name:       "http",
					Port:       app.Spec.Port,
					Protocol:   v1.ProtocolTCP,
					TargetPort: intstr.FromInt(int(app.Spec.Port)),
				},
			},
		},
		Status: v1.ServiceStatus{},
	}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, &srv, func() error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to create Service: %v", err)
	}
	return nil
}

func (r *ApplicationReconciler) reconcileDelete(ctx context.Context, app *minettodevv1alpha1.Application) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("removing service")
	err := r.removeService(ctx, app)
	if err != nil {
		return ctrl.Result{}, err
	}
	l.Info("removing deployment")
	err = r.removeDeployment(ctx, app)
	if err != nil {
		return ctrl.Result{}, err
	}
	l.Info("removing namespace")

	controllerutil.RemoveFinalizer(app, finalizer)
	return ctrl.Result{}, r.Update(ctx, app)
}

func (r *ApplicationReconciler) removeService(ctx context.Context, app *minettodevv1alpha1.Application) error {
	var srv v1.Service
	srvName := types.NamespacedName{Name: app.ObjectMeta.Name + "-service", Namespace: app.ObjectMeta.Name}
	if err := r.Get(ctx, srvName, &srv); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("unable to fetch Service: %v", err)
		}
	}
	if err := r.Delete(ctx, &srv); err != nil {
		return fmt.Errorf("unable to delete Service: %v", err)
	}
	return nil
}

func (r *ApplicationReconciler) removeDeployment(ctx context.Context, app *minettodevv1alpha1.Application) error {
	var depl appsv1.Deployment
	deplName := types.NamespacedName{Name: app.ObjectMeta.Name + "-deployment", Namespace: app.ObjectMeta.Name}
	if err := r.Get(ctx, deplName, &depl); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("unable to fetch Deployment: %v", err)
		}
	}
	if err := r.Delete(ctx, &depl); err != nil {
		return fmt.Errorf("unable to delete Deployment: %v", err)
	}
	return nil
}

func (r *ApplicationReconciler) removeNamespace(ctx context.Context, app *minettodevv1alpha1.Application) error {
	var ns v1.Namespace
	nsName := types.NamespacedName{Name: app.ObjectMeta.Name}
	if err := r.Get(ctx, nsName, &ns); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("unable to fetch Namespace: %v", err)
		}
	}
	if err := r.Delete(ctx, &ns); err != nil {
		return fmt.Errorf("unable to delete Namespace: %v", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&minettodevv1alpha1.Application{}).
		Complete(r)
}
