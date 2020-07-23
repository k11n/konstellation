/*


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

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components/prometheus"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/version"
)

const (
	prometheusName            = "prometheus"
	k8sName                   = "k8s"
	defaultScrapeInterval     = "15s"
	defaultEvaluationInterval = "15s"
	defaultRetentionPeriod    = "7d"
)

// ClusterConfigReconciler reconciles a ClusterConfig object
type ClusterConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k11n.dev,resources=clusterconfigs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=k11n.dev,resources=clusterconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheuses,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("clusterconfig", req.NamespacedName)

	// Fetch the ClusterConfig instance
	cc := &v1alpha1.ClusterConfig{}
	err := r.Client.Get(ctx, req.NamespacedName, cc)
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

	for _, target := range cc.Spec.Targets {
		if err := r.ensureNamespaceCreated(ctx, cc, target); err != nil {
			r.Log.Error(err, "could not create namespace", "namespace", target)
			return ctrl.Result{}, err
		}
	}

	if err := r.reconcilePrometheus(cc); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterConfig{}).
		Owns(&promv1.Prometheus{}).
		Complete(r)
}

func (r *ClusterConfigReconciler) ensureNamespaceCreated(ctx context.Context, cc *v1alpha1.ClusterConfig, target string) error {
	_, err := resources.GetNamespace(r.Client, target)
	if err == nil {
		return nil
	}

	// create a new one
	n := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: target,
			Labels: map[string]string{
				resources.IstioInjectLabel:   "enabled",
				resources.TargetLabel:        target,
				resources.KubeManagedByLabel: resources.Konstellation,
			},
		},
	}
	// ensures namespace is cleaned up after app target is
	err = ctrl.SetControllerReference(cc, &n, r.Scheme)
	if err != nil {
		return err
	}

	return r.Client.Create(ctx, &n)
}

func (r *ClusterConfigReconciler) reconcilePrometheus(cc *v1alpha1.ClusterConfig) error {
	// find default storage class
	var storageClass *storagev1.StorageClass
	err := resources.ForEach(r.Client, &storagev1.StorageClassList{}, func(obj interface{}) error {
		sc := obj.(storagev1.StorageClass)
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" || storageClass == nil {
			storageClass = &sc
		}
		return nil
	})

	if err != nil {
		return err
	}

	if storageClass == nil {
		return fmt.Errorf("No default storageClass defined")
	}

	// find prometheus component and get config
	compConf := cc.GetComponentConfig(prometheus.ComponentName)
	if compConf == nil {
		// component not yet installed
		return nil
	}

	pm := newPrometheus(compConf, storageClass.Name)
	op, err := resources.UpdateResource(r.Client, pm, cc, r.Scheme)
	if err != nil {
		return err
	}
	resources.LogUpdates(r.Log, op, "Updated Prometheus")
	return nil
}

func newPrometheus(config map[string]string, storageClass string) *promv1.Prometheus {
	volumeMode := corev1.PersistentVolumeFilesystem
	diskSize := prometheus.DefaultDiskSize
	if val, ok := config[prometheus.DiskSizeKey]; ok {
		if _, err := resource.ParseQuantity(val); err == nil {
			diskSize = val
		}
	}
	prom := &promv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: resources.KonSystemNamespace,
			Name:      k8sName,
			Labels: map[string]string{
				prometheusName: k8sName,
			},
		},
		Spec: promv1.PrometheusSpec{
			Affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      prometheusName,
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{k8sName},
										},
									},
								},
								Namespaces:  []string{resources.KonSystemNamespace},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				},
			},
			Alerting: &promv1.AlertingSpec{
				Alertmanagers: []promv1.AlertmanagerEndpoints{
					{
						Namespace: resources.KonSystemNamespace,
						Name:      "alertmanager-main",
						Port:      intstr.FromString("web"),
					},
				},
			},
			//BaseImage: "quay.io/prometheus/prometheus",
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			PodMonitorNamespaceSelector: &metav1.LabelSelector{},
			PodMonitorSelector:          &metav1.LabelSelector{},
			Replicas:                    pointer.Int32Ptr(2),
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse("400Mi"),
				},
			},
			Retention: defaultRetentionPeriod,
			RuleSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					prometheusName: k8sName,
					"role":         "alert-rules",
				},
			},
			EvaluationInterval: defaultEvaluationInterval,
			ScrapeInterval:     defaultScrapeInterval,
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup:      pointer.Int64Ptr(2000),
				RunAsNonRoot: pointer.BoolPtr(true),
				RunAsUser:    pointer.Int64Ptr(1000),
			},
			ServiceAccountName:              "prometheus-k8s",
			ServiceMonitorNamespaceSelector: &metav1.LabelSelector{},
			ServiceMonitorSelector:          &metav1.LabelSelector{},
			Storage: &promv1.StorageSpec{
				VolumeClaimTemplate: promv1.EmbeddedPersistentVolumeClaim{
					EmbeddedObjectMetadata: promv1.EmbeddedObjectMetadata{
						Name: "prometheus-pvc",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						VolumeMode:  &volumeMode,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								// TODO: make this configurable
								corev1.ResourceStorage: resource.MustParse(diskSize),
							},
						},
						StorageClassName: &storageClass,
					},
				},
			},
			Version: version.Prometheus,
		},
	}
	return prom
}
