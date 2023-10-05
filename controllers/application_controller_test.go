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

			// precisamos criar uma namespace no cluster
			ns := corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{Name: ApplicationNamespace},
			}
			Expect(k8sClient.Create(ctx, &ns)).Should(Succeed())

			//definimos uma Application
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
			//adicionamos o finalizer, conforme descrito no post anterior
			controllerutil.AddFinalizer(&app, finalizer)
			//garantimos que a criação não teve erros
			Expect(k8sClient.Create(ctx, &app)).Should(Succeed())
			//garantimos que o finalizer foi criado com sucesso
			Expect(controllerutil.ContainsFinalizer(&app, finalizer)).Should(BeTrue())

			// vamos agora verificar se o deployment foi criado com sucesso
			deplName := types.NamespacedName{Name: app.ObjectMeta.Name + "-deployment", Namespace: app.ObjectMeta.Name}
			createdDepl := &appsv1.Deployment{}

			//devido a natureza assíncrona do Kubernetes vamos fazer uso da função Eventually do Ginkgo
			//ele vai executar a função de acordo com o valor do intervalo, até que o timeout tenha terminado,
			//ou o resultado seja true
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deplName, createdDepl)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			//vamos verificar se os dados do Deployment foram criados de acordo com o esperado
			Expect(createdDepl.Spec.Template.Spec.Containers[0].Image).Should(Equal(app.Spec.Image))
			//o Application deve ser o Owner do Deployment
			Expect(createdDepl.ObjectMeta.OwnerReferences[0].Name).Should(Equal(app.Name))

			// vamos fazer o mesmo com o Service, garantindo que o controller criou conforme o esperado
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

			//vamos primeiro buscar a Application no cluster
			appName := types.NamespacedName{Name: ApplicationName, Namespace: ApplicationNamespace}
			app := minettodevv1alpha1.Application{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, appName, &app)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// vamos buscar o Deployment para garantir que os dados estão iguais aos do Application
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

			//vamos alterar a Application
			app.Spec.Image = "caddy:latest"
			Expect(k8sClient.Update(ctx, &app)).Should(Succeed())

			//vamos conferir se a alteração no Application se refletiu no Deployment
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
			//verifica se a exlusão aconteceu com sucesso
			Eventually(func() error {
				a := &minettodevv1alpha1.Application{}
				k8sClient.Get(context.Background(), appName, a)
				return k8sClient.Delete(context.Background(), a)
			}, timeout, interval).Should(Succeed())

			//garante que o Application não existe mais no cluster
			//este teste não é realmente necessário, pois o Delete aconteceu com sucesso
			//mantive este teste aqui apenas para fins didáticos
			Eventually(func() error {
				a := &minettodevv1alpha1.Application{}
				return k8sClient.Get(context.Background(), appName, a)
			}, timeout, interval).ShouldNot(Succeed())

			// de acordo com esta documentação : https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			// não podemos testar o garbage collection do cluster, para garantir que o Deployment e o Service criados foram removidos
			// mas no primeiro teste nós verificamos o ownership, então eles serão removidos de acordo com o esperado em um cluster real
		})
	})
})
