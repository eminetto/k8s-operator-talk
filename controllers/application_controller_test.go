package controllers

import (
	"context"
	"time"

	minettodevv1alpha1 "github.com/eminetto/k8s-operator-talk/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Application controller", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ApplicationName      = "test-app"
		ApplicationNamespace = "test-app"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating an Application", func() {
		It("Should create a Deployment and a Service", func() {
			By("An empty cluster")
			ctx := context.Background()

			// we need to create a namespace
			ns := corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{Name: ApplicationNamespace},
			}
			Expect(k8sClient.Create(ctx, &ns)).Should(Succeed())

			app := minettodevv1alpha1.Application{
				TypeMeta: v1.TypeMeta{
					Kind:       "Application",
					APIVersion: "v1alpha1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name: ApplicationName,
					// GenerateName: ApplicationNamespace,
					Namespace: ApplicationNamespace,
				},
				Spec: minettodevv1alpha1.ApplicationSpec{
					Image:    "nginx:latest",
					Replicas: 1,
					Port:     80,
				},
			}
			controllerutil.AddFinalizer(&app, finalizer)
			Expect(k8sClient.Create(ctx, &app)).Should(Succeed())
			Expect(controllerutil.ContainsFinalizer(&app, finalizer)).Should(BeTrue())

			lookupKey := types.NamespacedName{Name: ApplicationName, Namespace: ApplicationNamespace}
			createdApp := &minettodevv1alpha1.Application{}

			// We'll need to retry getting this newly created Application, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, lookupKey, createdApp)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdApp.Spec.Image).Should(Equal("nginx:latest"))

			// We'll need to retry getting this newly created Deployment, given that creation may not immediately happen.
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
			Expect(createdDepl.ObjectMeta.OwnerReferences[0].Name).Should(Equal(app.Name))

			// We'll need to retry getting this newly created Deployment, given that creation may not immediately happen.
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
			By("An Application change")
			ctx := context.Background()

			appName := types.NamespacedName{Name: ApplicationName, Namespace: ApplicationNamespace}
			app := minettodevv1alpha1.Application{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, appName, &app)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// We'll need to retry getting this newly created Deployment, given that creation may not immediately happen.
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

			app.Spec.Image = "caddy:latest"
			Expect(k8sClient.Update(ctx, &app)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, appName, &app)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(app.Spec.Image).Should(Equal("caddy:latest"))

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplName, createdDepl)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdDepl.Spec.Template.Spec.Containers[0].Image).Should(Equal(app.Spec.Image))
		})
	})

	Context("When deleting an Application", func() {
		It("Should delete the Deployment and Service", func() {
			// Delete
			appName := types.NamespacedName{Name: ApplicationName, Namespace: ApplicationNamespace}
			By("Expecting to delete successfully")
			Eventually(func() error {
				a := &minettodevv1alpha1.Application{}
				k8sClient.Get(context.Background(), appName, a)
				return k8sClient.Delete(context.Background(), a)
			}, timeout, interval).Should(Succeed())

			By("Expecting to delete finish")
			Eventually(func() error {
				a := &minettodevv1alpha1.Application{}
				return k8sClient.Get(context.Background(), appName, a)
			}, timeout, interval).ShouldNot(Succeed())

			// According with this documentation: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			// we can't test the garbage collection, to garantee that the created Deployment and Service was removed
			// but in the lines 85 e 99 we test the ownership, so everthing will be removed properly

		})
	})
})
