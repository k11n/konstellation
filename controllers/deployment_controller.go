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

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	"github.com/thoas/go-funk"
	istio "istio.io/client-go/pkg/apis/networking/v1beta1"
	autoscale "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

// DeploymentReconciler reconciles an AppTarget and other resources
type DeploymentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k11n.dev,resources=appconfigs;apptargets;appreleases;builds;ingressrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k11n.dev,resources=apptargets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=destinationrules;gateways;virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules;servicemonitors;podmonitors,verbs=get;list;watch;create;update;patch;delete

func (r *DeploymentReconciler) Reconcile(req ctrl.Request) (res ctrl.Result, err error) {
	ctx := context.Background()

	at, err := resources.GetAppTarget(r.Client, req.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return
	}

	// copy status to detect changes
	atStatus := at.Status.DeepCopy()

	// figure out configs
	configMap, err := r.reconcileConfigMap(ctx, at)
	if err != nil {
		return
	}

	// create releases and figure out traffic split
	releases, arRes, err := r.reconcileAppReleases(ctx, at, configMap)
	if err != nil {
		return
	}
	if arRes != nil {
		if arRes.Requeue {
			res.Requeue = arRes.Requeue
		}
		if arRes.RequeueAfter != 0 {
			res.RequeueAfter = arRes.RequeueAfter
		}
	}

	// see which releases we need to autoscale
	err = r.reconcileAutoScaler(ctx, at, releases)
	if err != nil {
		return
	}

	// reconcile Service
	service, err := r.reconcileService(ctx, at)
	if err != nil {
		return
	}

	// reconcile prometheus setup
	if err = r.reconcilePrometheusServiceMonitor(ctx, at); err != nil {
		return
	}
	if err = r.reconcilePrometheusRules(ctx, at); err != nil {
		return
	}

	// filter only releases with traffic
	activeReleases := funk.Filter(releases, func(ar *v1alpha1.AppRelease) bool {
		return ar.Spec.TrafficPercentage > 0
	}).([]*v1alpha1.AppRelease)
	err = r.reconcileDestinationRule(ctx, at, service, activeReleases)
	if err != nil {
		return
	}

	err = r.reconcileVirtualService(ctx, at, service, activeReleases)
	if err != nil {
		return
	}

	err = r.reconcileIngressRequest(ctx, at)
	if err != nil {
		return
	}

	// update at status
	if !apiequality.Semantic.DeepEqual(atStatus, at.Status) {
		// reload apptarget and update status
		status := at.Status
		at, err = resources.GetAppTarget(r.Client, at.Name)
		if err != nil {
			return
		}
		at.Status = status
		err = r.Client.Status().Update(context.TODO(), at)
	}
	return
}

func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	configWatcher := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(configMapObject handler.MapObject) []ctrl.Request {
			requests := []ctrl.Request{}
			// check which apps
			appConfig := configMapObject.Object.(*v1alpha1.AppConfig)

			if appConfig.Type == v1alpha1.ConfigTypeApp {
				targets, err := resources.GetAppTargets(mgr.GetClient(), appConfig.GetAppName())
				if err != nil {
					return requests
				}
				desiredTarget := appConfig.Labels[v1alpha1.TargetLabel]

				for _, target := range targets {
					if desiredTarget != "" && desiredTarget != target.Spec.Target {
						// skip if it's a target specific config change
						continue
					}
					requests = append(requests, ctrl.Request{
						NamespacedName: types.NamespacedName{
							Namespace: target.Namespace,
							Name:      target.Name,
						},
					})
				}
			} else if appConfig.Type == v1alpha1.ConfigTypeShared {
				// load all app targets and see which ones use this config
				resources.ForEach(mgr.GetClient(), &v1alpha1.AppTarget{}, func(item interface{}) error {
					at := item.(*v1alpha1.AppTarget)
					for _, conf := range at.Spec.Configs {
						if conf == appConfig.GetSharedName() {
							requests = append(requests, ctrl.Request{
								NamespacedName: types.NamespacedName{
									Namespace: at.Namespace,
									Name:      at.Name,
								},
							})
							break
						}
					}
					return nil
				})
			}

			return requests
		}),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.AppTarget{}).
		Owns(&v1alpha1.AppRelease{}).
		Owns(&v1alpha1.IngressRequest{}).
		Owns(&corev1.Service{}).
		Owns(&istio.VirtualService{}).
		Owns(&autoscale.HorizontalPodAutoscaler{}).
		Watches(&source.Kind{Type: &v1alpha1.AppConfig{}}, configWatcher, builder.WithPredicates(predicate.Funcs{
			// grab ingress events so that we could update its status
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			CreateFunc:  func(e event.CreateEvent) bool { return true },
			UpdateFunc:  func(e event.UpdateEvent) bool { return true },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		})).
		Complete(r)
}

func (r *DeploymentReconciler) reconcileConfigMap(ctx context.Context, at *v1alpha1.AppTarget) (configMap *corev1.ConfigMap, err error) {
	// grab app release for this app
	ac, err := resources.GetMergedConfigForType(r.Client, v1alpha1.ConfigTypeApp, at.Spec.App, at.Spec.Target)
	if err != nil {
		return
	}

	// find other configmaps
	sharedConfigs := make([]*v1alpha1.AppConfig, 0, len(at.Spec.Configs))
	for _, config := range at.Spec.Configs {
		sc, cErr := resources.GetMergedConfigForType(r.Client, v1alpha1.ConfigTypeShared, config, at.Spec.Target)
		if cErr != nil {
			// skip this config and continue
			r.Log.Error(cErr, "Could not find shared config", "app", at.Spec.App,
				"target", at.Spec.Target, "config", config)
			continue
		}
		sharedConfigs = append(sharedConfigs, sc)
	}

	// check if existing configmap with the hash
	if ac == nil && len(sharedConfigs) == 0 {
		// no config maps needed
		return
	}
	configMap = resources.CreateConfigMap(at.Spec.App, ac, sharedConfigs)
	for key, val := range labelsForAppTarget(at) {
		configMap.Labels[key] = val
	}
	_, err = resources.GetConfigMap(r.Client, at.TargetNamespace(), configMap.Name)
	if errors.IsNotFound(err) {
		r.Log.Info("Creating ConfigMap", "app", at.Spec.App, "target", at.Spec.Target)
		// create new
		configMap.Namespace = at.TargetNamespace()
		err = r.Client.Create(ctx, configMap)
	}
	return
}

func (r *DeploymentReconciler) reconcilePrometheusServiceMonitor(ctx context.Context, at *v1alpha1.AppTarget) error {
	needsServiceMonitor := true
	if !at.NeedsService() {
		needsServiceMonitor = false
	}
	prom := at.Spec.Prometheus
	if prom == nil {
		needsServiceMonitor = false
	} else if len(prom.Endpoints) == 0 {
		needsServiceMonitor = false
	}

	if !needsServiceMonitor {
		existing := &promv1.ServiceMonitor{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: at.TargetNamespace(), Name: at.Spec.App}, existing)
		if errors.IsNotFound(err) {
			// does not exist, perfect
			return nil
		} else if err != nil {
			return err
		}

		// delete existing service monitor
		r.Log.Info("deleting ServiceMonitor", "appTarget", at.Name)
		if err = r.Client.Delete(ctx, existing); err != nil {
			return err
		}
		return nil
	}

	// create or update service monitor
	sm := newServiceMonitorForAppTarget(at)
	op, err := resources.UpdateResource(r.Client, sm, at, r.Scheme)
	if err != nil {
		return err
	}

	resources.LogUpdates(r.Log, op, "updated ServiceMonitor",
		"appTarget", at.Name, "port", sm.Spec.Endpoints[0].Port)

	return nil
}

func (r *DeploymentReconciler) reconcilePrometheusRules(ctx context.Context, at *v1alpha1.AppTarget) error {
	needsRules := true
	prom := at.Spec.Prometheus
	if prom == nil {
		needsRules = false
	} else if len(prom.Rules) == 0 {
		needsRules = false
	}

	if !needsRules {
		// delete existing prometheus rule
		existing := &promv1.PrometheusRule{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: at.TargetNamespace(), Name: at.Spec.App}, existing)
		if errors.IsNotFound(err) {
			// does not exist, perfect
			return nil
		} else if err != nil {
			return err
		}

		// delete existing service monitor
		r.Log.Info("deleting PrometheusRule", "appTarget", at.Name)
		if err = r.Client.Delete(ctx, existing); err != nil {
			return err
		}
		return nil
	}

	// create or update
	pr := newPromRuleForAppTarget(at)
	op, err := resources.UpdateResource(r.Client, pr, at, r.Scheme)
	if err != nil {
		return err
	}

	resources.LogUpdates(r.Log, op, "updated PrometheusRule",
		"appTarget", at.Name, "numRules", len(pr.Spec.Groups[0].Rules))

	return nil
}

func newServiceMonitorForAppTarget(at *v1alpha1.AppTarget) *promv1.ServiceMonitor {
	sm := &promv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: at.TargetNamespace(),
			Name:      at.Spec.App,
			Labels:    labelsForAppTarget(at),
		},
		Spec: promv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					resources.AppLabel:    at.Spec.App,
					resources.TargetLabel: at.Spec.Target,
				},
			},
			NamespaceSelector: promv1.NamespaceSelector{
				MatchNames: []string{at.TargetNamespace()},
			},
			Endpoints: at.Spec.Prometheus.Endpoints,
		},
	}
	return sm
}

func newPromRuleForAppTarget(at *v1alpha1.AppTarget) *promv1.PrometheusRule {
	pr := &promv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: at.TargetNamespace(),
			Name:      at.Spec.App,
			Labels:    labelsForAppTarget(at),
		},
		Spec: promv1.PrometheusRuleSpec{
			Groups: []promv1.RuleGroup{
				{
					Name:  at.Spec.App,
					Rules: at.Spec.Prometheus.Rules,
				},
			},
		},
	}
	return pr
}

func labelsForAppTarget(at *v1alpha1.AppTarget) map[string]string {
	return map[string]string{
		resources.AppLabel:           at.Spec.App,
		resources.TargetLabel:        at.Spec.Target,
		resources.KubeManagedByLabel: resources.Konstellation,
		resources.KubeAppLabel:       at.Spec.App,
	}
}

func selectorsForAppTarget(at *v1alpha1.AppTarget) map[string]string {
	return map[string]string{
		resources.AppLabel:    at.Spec.App,
		resources.TargetLabel: at.Spec.Target,
	}
}
