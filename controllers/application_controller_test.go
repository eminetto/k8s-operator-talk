package controllers

import (
	"context"
	"time"

	minettodevv1alpha1 "github.com/eminetto/k8s-operator-talk/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Define utility constants for object names and testing timeouts/durations and intervals.
const (
	ApplicationName      = "test-app"
	ApplicationNamespace = "test-app"

	timeout  = time.Second * 10
	duration = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("Application controller", func() {
	Context("When creating an Application", func() {
		It("Should create a Deployment and a Service", func() {
			ctx := context.Background()

			// we need to create a namespace in the cluster
			ns := corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{Name: ApplicationNamespace},
			}
			Expect(k8sClient.Create(ctx, &ns)).Should(Succeed())

			// we define an Application
			app := minettodevv1alpha1.Application{
				TypeMeta: v1.TypeMeta{
					Kind:       "Application",
					APIVersion: "v1alpha1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      ApplicationName,
					Namespace: ApplicationNamespace,
				},
				Spec: minettodevv1alpha1.ApplicationSpec{
					Image:    "nginx:latest",
					Replicas: 1,
					Port:     80,
				},
			}
			// we added the finalizer, as described in the previous post
			controllerutil.AddFinalizer(&app, finalizer)
			// we guarantee that the creation was error-free
			Expect(k8sClient.Create(ctx, &app)).Should(Succeed())
			// we guarantee that the finalizer was created successfully
			Expect(controllerutil.ContainsFinalizer(&app, finalizer)).Should(BeTrue())

			// let's now check if the deployment was created successfully
			deplName := types.NamespacedName{Name: app.ObjectMeta.Name + "-deployment", Namespace: app.ObjectMeta.Name}
			createdDepl := &appsv1.Deployment{}

			// due to the asynchronous nature of Kubernetes, we will
			// make use of Ginkgo's Eventually function. It will
			// execute the function according to the interval value,
			// until the timeout has ended, or the result is true
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplName, createdDepl)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			// let's check if the Deployment data was created as expected
			Expect(createdDepl.Spec.Template.Spec.Containers[0].Image).Should(Equal(app.Spec.Image))
			// the Application must be the Owner of the Deployment
			Expect(createdDepl.ObjectMeta.OwnerReferences[0].Name).Should(Equal(app.Name))

			// let's do the same with the Service, ensuring that the controller created it as expected
			srvName := types.NamespacedName{Name: app.ObjectMeta.Name + "-service", Namespace: app.ObjectMeta.Name}
			createdSrv := &corev1.Service{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, srvName, createdSrv)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdSrv.Spec.Ports[0].TargetPort).Should(Equal(intstr.FromInt(int(app.Spec.Port))))
			Expect(createdDepl.ObjectMeta.OwnerReferences[0].Name).Should(Equal(app.Name))
		})
	})

	Context("When updating an Application", func() {
		It("Should update the Deployment", func() {
			ctx := context.Background()

			// let's first retrieve the Application in the cluster
			appName := types.NamespacedName{Name: ApplicationName, Namespace: ApplicationNamespace}
			app := minettodevv1alpha1.Application{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, appName, &app)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// let's retrieve the Deployment to ensure that the data is the same as the Application
			deplName := types.NamespacedName{Name: app.ObjectMeta.Name + "-deployment", Namespace: app.ObjectMeta.Name}
			createdDepl := &appsv1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplName, createdDepl)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdDepl.Spec.Template.Spec.Containers[0].Image).Should(Equal(app.Spec.Image))

			// let's change the Application
			app.Spec.Image = "caddy:latest"
			Expect(k8sClient.Update(ctx, &app)).Should(Succeed())

			// let's check if the change in the Application was reflected in the Deployment
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplName, createdDepl)
				if err != nil {
					return false
				}
				if createdDepl.Spec.Template.Spec.Containers[0].Image == "caddy:latest" {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When deleting an Application", func() {
		It("Should delete the Deployment and Service", func() {
			appName := types.NamespacedName{Name: ApplicationName, Namespace: ApplicationNamespace}
			// checks whether the deletion was successful
			Eventually(func() error {
				a := &minettodevv1alpha1.Application{}
				k8sClient.Get(context.Background(), appName, a)
				return k8sClient.Delete(context.Background(), a)
			}, timeout, interval).Should(Succeed())

			// ensures that the Application no longer exists in the cluster
			// this test is not really necessary as the Delete happened successfully
			// I kept this test here for educational purposes only.
			Eventually(func() error {
				a := &minettodevv1alpha1.Application{}
				return k8sClient.Get(context.Background(), appName, a)
			}, timeout, interval).ShouldNot(Succeed())

			// according to this documentation: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			// we cannot test the cluster's garbage collection to ensure that the Deployment and Service created were removed,
			// but in the first test we check the ownership, so they will be removed as expected in a real cluster
		})
	})
})
