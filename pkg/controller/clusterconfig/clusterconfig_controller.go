package clusterconfig

import (
	"context"
	"fmt"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/components/prometheus"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/version"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller.ClusterConfig")

const (
	prometheusName        = "prometheus"
	k8sName               = "k8s"
	defaultScrapeInterval = "15s"
)

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClusterConfig{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clusterconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ClusterConfig
	err = c.Watch(&source.Kind{Type: &v1alpha1.ClusterConfig{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileClusterConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileClusterConfig{}

// ReconcileClusterConfig reconciles a ClusterConfig object
type ReconcileClusterConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ClusterConfig object and makes changes based on the state read
// and what is in the ClusterConfig.Spec
func (r *ReconcileClusterConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the ClusterConfig instance
	cc := &v1alpha1.ClusterConfig{}
	err := r.client.Get(context.TODO(), request.NamespacedName, cc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	for _, target := range cc.Spec.Targets {
		if err := r.ensureNamespaceCreated(cc, target); err != nil {
			log.Error(err, "could not create namespace", "namespace", target)
			return reconcile.Result{}, err
		}
	}

	if err := r.reconcilePrometheus(cc); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileClusterConfig) ensureNamespaceCreated(cc *v1alpha1.ClusterConfig, target string) error {
	_, err := resources.GetNamespace(r.client, target)
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
	err = controllerutil.SetControllerReference(cc, &n, r.scheme)
	if err != nil {
		return err
	}

	return r.client.Create(context.TODO(), &n)
}

func (r *ReconcileClusterConfig) reconcilePrometheus(cc *v1alpha1.ClusterConfig) error {
	// find default storage class
	var storageClass *storagev1.StorageClass
	err := resources.ForEach(r.client, &storagev1.StorageClassList{}, func(obj interface{}) error {
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
	op, err := resources.UpdateResource(r.client, pm, cc, r.scheme)
	if err != nil {
		return err
	}
	resources.LogUpdates(log, op, "Updated Prometheus")
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
			RuleSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					prometheusName: k8sName,
					"role":         "alert-rules",
				},
			},
			ScrapeInterval: defaultScrapeInterval,
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup:      pointer.Int64Ptr(2000),
				RunAsNonRoot: pointer.BoolPtr(true),
				RunAsUser:    pointer.Int64Ptr(1000),
			},
			ServiceAccountName:              "prometheus-k8s",
			ServiceMonitorNamespaceSelector: &metav1.LabelSelector{},
			ServiceMonitorSelector:          &metav1.LabelSelector{},
			Storage: &promv1.StorageSpec{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prometheus-pvc",
						Namespace: resources.KonSystemNamespace,
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
			Version: version.PrometheusVersion,
		},
	}
	return prom
}
