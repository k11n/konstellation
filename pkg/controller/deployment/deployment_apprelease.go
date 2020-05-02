package deployment

import (
	"context"
	"fmt"
	"time"

	autoscalev2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
)

const (
	rampIncrement = 0.25
)

func (r *ReconcileDeployment) reconcileAppReleases(at *v1alpha1.AppTarget, configMap *corev1.ConfigMap) (releases []*v1alpha1.AppRelease, res *reconcile.Result, err error) {
	// find last N builds
	builds, err := resources.GetBuildsByImage(r.client, at.Spec.BuildRegistry, at.Spec.BuildImage, 6)
	if err != nil {
		return
	}

	// create new releases if needed
	releases, err = resources.GetAppReleases(r.client, at.Spec.App, at.Spec.Target, 50)
	if err != nil {
		return
	}

	// keep track of builds that we already have a release for, those can be ignored
	existingReleases := map[string]*v1alpha1.AppRelease{}
	for _, ar := range releases {
		existingReleases[ar.Spec.Build] = ar
	}

	// create releases for new builds
	for i, b := range builds {
		ar := existingReleases[b.Name]
		if ar != nil {
			if configMap != nil && i == 0 {
				// the latest build requires configMap hash to match too
				// otherwise we should create new release on config changes
				if ar.Spec.Config == configMap.Name {
					continue
				} else {
					log.Info("config changed, creating new release", "configMap", configMap.Name,
						"build", b.Name, "appRelease", ar.Name)
				}
			} else {
				continue
			}
		}

		ar = appReleaseForTarget(at, &b, configMap)
		releases = append(releases, ar)
	}

	// sort releases and determine traffic and latest
	resources.SortAppReleasesByLatest(releases)

	releasesCopy := make([]*v1alpha1.AppRelease, 0, len(releases))
	for _, ar := range releases {
		releasesCopy = append(releasesCopy, ar.DeepCopy())
	}

	// determine target release and traffic split
	res, err = r.deployReleases(at, releases)
	if err != nil {
		return
	}

	// save new resources and any modified existing ones
	for i, ar := range releases {
		if !ar.CreationTimestamp.IsZero() && apiequality.Semantic.DeepEqual(ar, releasesCopy[i]) {
			continue
		}

		// see if we can find the existing item
		var op controllerutil.OperationResult
		op, err = resources.UpdateResource(r.client, ar, at, r.scheme)
		if err != nil {
			return
		}

		resources.LogUpdates(log, op, "Updated AppRelease", "appTarget", at.Name, "release", ar.Name)
	}

	// delete all except for the last 6
	if len(releases) > 6 {
		toDelete := releases[6:]
		releases = releases[:6]

		for _, ar := range toDelete {
			err = r.client.Delete(context.TODO(), ar)
			if err != nil {
				return
			}
		}
	}
	return
}

/**
 * Determine current releases and flip release switch. TODO: figure out how to signal requeue
 */
func (r *ReconcileDeployment) deployReleases(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) (res *reconcile.Result, err error) {
	logger := log.WithValues("appTarget", at.Name)

	if len(releases) == 0 {
		err = fmt.Errorf("cannot deploy empty releases")
		return
	}
	var targetRelease *v1alpha1.AppRelease
	var activeRelease *v1alpha1.AppRelease

	for _, ar := range releases {
		if ar.Spec.Role == v1alpha1.ReleaseRoleActive {
			activeRelease = ar
		} else if ar.Spec.Role == v1alpha1.ReleaseRoleTarget {
			targetRelease = ar
		}
	}
	// target and active are the same release
	if targetRelease == nil {
		targetRelease = activeRelease
	}

	if activeRelease != nil && targetRelease != nil {
		// only update when it's time to, otherwise requeue
		timeDelta := time.Now().Sub(at.Status.DeployUpdatedAt.Time)
		if timeDelta < at.Spec.Probes.GetReadinessTimeout() {
			res = &reconcile.Result{
				RequeueAfter: at.Spec.Probes.GetReadinessTimeout() - timeDelta,
			}
			return
		}
	}

	// first deploy, turn on immediately
	if activeRelease == nil {
		activeRelease = releases[0]
		targetRelease = activeRelease
		logger.Info("Deploying initial release", "release", activeRelease.Name)
	} else {
		// TODO: don't deploy additional builds when outside of schedule
		// see if there's a new target release (try to deploy latest if possible)
		// TODO: check if autorelease is enabled for this target..
		newTarget := resources.FirstAvailableRelease(releases)
		if newTarget != nil {
			if targetRelease != newTarget {
				var previousTarget string
				if targetRelease != nil {
					previousTarget = targetRelease.Name
				}
				logger.Info("Setting new target release", "target", newTarget.Name, "previousTarget", previousTarget)
			}
			targetRelease = newTarget
		}
	}

	desiredInstances := at.DesiredInstances()
	targetTrafficPercentage := targetRelease.Spec.TrafficPercentage
	if targetRelease == activeRelease {
		targetTrafficPercentage = 100
		targetRelease.Spec.NumDesired = desiredInstances
	} else {
		// increase by up to rampIncrement
		maxIncrement := int32(float32(desiredInstances) * rampIncrement)
		if maxIncrement < 1 {
			maxIncrement = 1
		}
		targetInstances := targetRelease.Status.NumAvailable + maxIncrement
		if targetInstances > desiredInstances {
			targetInstances = desiredInstances
		}
		if targetRelease.Spec.NumDesired < targetInstances {
			logger.Info("Increasing pods", "release", targetRelease.Name,
				"numDesired", targetRelease.Spec.NumDesired, "newNumDesired", targetInstances)
			targetRelease.Spec.NumDesired = targetInstances
		}

		ratioDeployed := float32(targetRelease.Status.NumAvailable) / float32(desiredInstances)
		if ratioDeployed > 1 {
			ratioDeployed = 1
		}
		targetTrafficPercentage = int32(ratioDeployed * 100)

		// should not increment more than ramp increment% at a time
		rampPercentage := int32(100 * rampIncrement)
		if targetTrafficPercentage-targetRelease.Spec.TrafficPercentage > rampPercentage {
			targetTrafficPercentage = targetRelease.Spec.TrafficPercentage + rampPercentage
		}

		if targetTrafficPercentage < 100 || targetTrafficPercentage <= targetRelease.Spec.TrafficPercentage {
			// before we are fully ramped, requeue to check again
			// we use <= current traffic ramp since the actual traffic may be less than 100% (due to new canaries/etc)
			res = &reconcile.Result{
				RequeueAfter: at.Spec.Probes.GetReadinessTimeout(),
			}
		} else {
			// fully deployed, update active roles and we are done
			activeRelease = targetRelease
			logger.Info("Target fully deployed, marking as active", "release", targetRelease.Name)
		}
	}

	// now update state on releases
	earlierThanActive := false
	// all traffic percentage so far, active should get what remains
	var totalTraffic int32
	for _, ar := range releases {
		if ar == activeRelease {
			ar.Spec.Role = v1alpha1.ReleaseRoleActive
			earlierThanActive = true
			if ar == targetRelease {
				ar.Spec.TrafficPercentage = targetTrafficPercentage
			} else {
				// scale down pods as a proportion of traffic
				instanceCount := int32(float32(desiredInstances) * float32(ar.Spec.TrafficPercentage) / 100)
				if instanceCount < 1 && ar.Spec.TrafficPercentage != 0 {
					instanceCount = 1
				}
				ar.Spec.NumDesired = instanceCount
				ar.Spec.TrafficPercentage = 100 - targetTrafficPercentage
			}
		} else if ar == targetRelease {
			ar.Spec.Role = v1alpha1.ReleaseRoleTarget
			if ar.Spec.TrafficPercentage != targetTrafficPercentage {
				logger.Info("Updating traffic", "release", ar.Name, "traffic", targetTrafficPercentage,
					"lastTraffic", ar.Spec.TrafficPercentage)
			}
			ar.Spec.TrafficPercentage = targetTrafficPercentage

		} else {
			// TODO: update traffic for canaries
			ar.Spec.Role = v1alpha1.ReleaseRoleNone
			ar.Spec.TrafficPercentage = 0

			// other releases should run at 0 or minimal
			if earlierThanActive {
				ar.Spec.NumDesired = 0
			} else {
				ar.Spec.NumDesired = 1
			}
		}
		totalTraffic += ar.Spec.TrafficPercentage
	}

	// if we have over 100%, then lower target until we are within threshold
	if totalTraffic != 100 {
		targetRelease.Spec.TrafficPercentage -= totalTraffic - 100
	}

	at.Status.ActiveRelease = activeRelease.Name
	at.Status.TargetRelease = targetRelease.Name
	at.Status.DeployUpdatedAt = metav1.Now()

	if !activeRelease.CreationTimestamp.IsZero() {
		at.Status.NumDesired = activeRelease.Status.NumDesired
		at.Status.NumReady = activeRelease.Status.NumReady
		at.Status.NumAvailable = activeRelease.Status.NumAvailable
	}
	return
}

/**
 * Configure autoscaler for the active release. If active release has changed, then delete and recreate scaler
 * when active release and target release aren't the same, we will want to pause the scaler
 * reconciler auto updates appTarget status with current number desired by scaler
 */
func (r *ReconcileDeployment) reconcileAutoScaler(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) error {
	// find active release and target
	var activeRelease, targetRelease *v1alpha1.AppRelease
	for _, ar := range releases {
		if ar.Spec.Role == v1alpha1.ReleaseRoleActive {
			activeRelease = ar
		} else if ar.Spec.Role == v1alpha1.ReleaseRoleTarget {
			targetRelease = ar
		}
	}

	ctx := context.TODO()

	needsScaler := activeRelease != nil && targetRelease == nil
	// find all existing scalers
	scalerList := autoscalev2beta2.HorizontalPodAutoscalerList{}
	err := r.client.List(ctx, &scalerList, client.MatchingLabels(labelsForAppTarget(at)))
	if err != nil {
		return err
	}

	// remove all the existing scalers and exit
	if !needsScaler {
		for _, scaler := range scalerList.Items {
			log.Info("Deleting unused autoscaler", "appTarget", at.Name)
			if err = r.client.Delete(ctx, &scaler); err != nil {
				return err
			}
		}
		return nil
	}

	scaler := newAutoscalerForAppTarget(at, activeRelease)
	// see if any of the existing scalers match the current template
	for _, s := range scalerList.Items {
		if s.Labels[resources.AppReleaseLabel] == scaler.Labels[resources.AppReleaseLabel] {
			scaler = &s
		} else {
			// delete the other ones (there should be only one scaler at any time
			log.Info("Deleting autoscaler for old releases", "appTarget", at.Name)
			if err := r.client.Delete(ctx, &s); err != nil {
				return err
			}
		}
	}

	op, err := resources.UpdateResource(r.client, scaler, at, r.scheme)
	if err != nil {
		return err
	}
	resources.LogUpdates(log, op, "Updated autoscaler", "appTarget", at.Name)

	// update status
	at.Status.LastScaledAt = scaler.Status.LastScaleTime

	return err
}

func appReleaseForTarget(at *v1alpha1.AppTarget, build *v1alpha1.Build, configMap *corev1.ConfigMap) *v1alpha1.AppRelease {
	labels := labelsForAppTarget(at)
	labels[resources.ConfigHashLabel] = configMap.Name
	ar := &v1alpha1.AppRelease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: at.TargetNamespace(),
			Name:      fmt.Sprintf("%s-%s", at.Spec.App, time.Now().Format("20060102-150405")),
			Labels:    labels,
		},
		Spec: v1alpha1.AppReleaseSpec{
			App:       at.Spec.App,
			Target:    at.Spec.Target,
			Build:     build.Name,
			Role:      v1alpha1.ReleaseRoleNone,
			Ports:     at.Spec.Ports,
			Command:   at.Spec.Command,
			Args:      at.Spec.Args,
			Resources: at.Spec.Resources,
			Probes:    at.Spec.Probes,
		},
	}
	if configMap != nil {
		ar.Spec.Config = configMap.Name
	}
	return ar
}

func newAutoscalerForAppTarget(at *v1alpha1.AppTarget, ar *v1alpha1.AppRelease) *autoscalev2beta2.HorizontalPodAutoscaler {
	minReplicas := at.Spec.Scale.Min
	maxReplicas := at.Spec.Scale.Max
	if minReplicas == 0 {
		minReplicas = 1
	}
	if maxReplicas < minReplicas {
		maxReplicas = minReplicas
	}
	var metrics []autoscalev2beta2.MetricSpec
	if at.Spec.Scale.TargetCPUUtilization > 0 {
		metrics = append(metrics, autoscalev2beta2.MetricSpec{
			Type: autoscalev2beta2.ResourceMetricSourceType,
			Resource: &autoscalev2beta2.ResourceMetricSource{
				Name: "cpu",
				Target: autoscalev2beta2.MetricTarget{
					Type:               autoscalev2beta2.UtilizationMetricType,
					AverageUtilization: &at.Spec.Scale.TargetCPUUtilization,
				},
			},
		})
	}

	labels := labelsForAppTarget(at)
	labels[resources.AppReleaseLabel] = ar.Name
	autoscaler := autoscalev2beta2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-scaler", at.Spec.App),
			Namespace: at.TargetNamespace(),
			Labels:    labels,
		},
		Spec: autoscalev2beta2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalev2beta2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       ar.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
		},
	}
	return &autoscaler
}
