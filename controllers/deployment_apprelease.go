package controllers

import (
	"context"
	"fmt"
	"time"

	autoscale "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/files"
)

const (
	rampIncrement = 0.25
	// keep at least 10 releases and everything in last 48 hours
	numReleasesToKeep  = 10
	releaseHoursToKeep = 48
)

func (r *DeploymentReconciler) reconcileAppReleases(ctx context.Context, at *v1alpha1.AppTarget, configMap *corev1.ConfigMap) (releases []*v1alpha1.AppRelease, res *ctrl.Result, err error) {
	// find the named build for the app
	build, err := resources.GetBuildByName(r.Client, at.Spec.Build)
	if err != nil {
		return
	}

	// find all existing releases
	err = resources.ForEach(r.Client, &v1alpha1.AppReleaseList{}, func(item interface{}) error {
		release := item.(v1alpha1.AppRelease)
		releases = append(releases, &release)
		return nil
	}, client.MatchingLabels{
		resources.AppLabel:    at.Spec.App,
		resources.TargetLabel: at.Spec.Target,
	})
	if err != nil {
		return
	}

	// do we already have a release for this appTargetHash and configmap combination?
	// if not we'd want to create a new release
	var existingRelease *v1alpha1.AppRelease
	for _, ar := range releases {
		if ar.Labels[v1alpha1.AppTargetHash] != at.GetHash() {
			// not the current release
			continue
		}

		if configMap == nil || configMap.Name == ar.Spec.Config {
			existingRelease = ar
			break
		}
	}

	// create releases for new builds
	if existingRelease == nil {
		configName := ""
		if configMap != nil {
			configName = configMap.Name
		}
		r.Log.Info("config changed, creating new release", "configMap", configName,
			"build", build.Name)
		ar := appReleaseForTarget(at, build, configMap)
		releases = append(releases, ar)
	}

	// sort releases and determine traffic and latest
	resources.SortAppReleasesByLatest(releases)

	releasesCopy := make([]*v1alpha1.AppRelease, 0, len(releases))
	for _, ar := range releases {
		releasesCopy = append(releasesCopy, ar.DeepCopy())
	}

	// determine target release and traffic split
	res, err = r.deployReleases(ctx, at, releases)
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
		op, err = resources.UpdateResource(r.Client, ar, at, r.Scheme)
		if err != nil {
			return
		}

		resources.LogUpdates(r.Log, op, "Updated AppRelease", "appTarget", at.Name, "release", ar.Name)
	}

	// delete older releases
	if len(releases) > numReleasesToKeep {
		toDelete := releases[numReleasesToKeep:]
		releases = releases[:numReleasesToKeep]

		for _, ar := range toDelete {
			if time.Since(ar.CreationTimestamp.Time) < releaseHoursToKeep*time.Hour {
				// skip deletion of newer releases
				continue
			}
			err = r.Client.Delete(ctx, ar)
			if err != nil {
				return
			}
		}
	}
	return
}

/**
 * Determine current releases and flip release switch
 */
func (r *DeploymentReconciler) deployReleases(ctx context.Context, at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) (res *ctrl.Result, err error) {
	// need a way to test this controller
	logger := r.Log.WithValues("appTarget", at.Name)

	if len(releases) == 0 {
		err = fmt.Errorf("cannot deploy empty releases")
		return
	}
	var targetRelease *v1alpha1.AppRelease
	var activeRelease *v1alpha1.AppRelease
	hasChanges := false

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

	if activeRelease != nil && targetRelease != nil && activeRelease != targetRelease {
		// only update when it's time to, otherwise requeue
		timeDelta := time.Now().Sub(at.Status.DeployUpdatedAt.Time)
		if timeDelta < at.Spec.Probes.GetReadinessTimeout() {
			res = &ctrl.Result{
				RequeueAfter: at.Spec.Probes.GetReadinessTimeout() - timeDelta,
			}
			logger.Info("waiting for next reconcile")
			return
		}
	}

	firstDeployableRelease := resources.GetFirstDeployableRelease(releases)
	if firstDeployableRelease == nil {
		// can't be deployed
		return
	}

	// choose target release
	if activeRelease == nil {
		// first deploy, turn on immediately
		activeRelease = firstDeployableRelease
		targetRelease = activeRelease
		logger.Info("Deploying initial release", "release", activeRelease.Name)
		hasChanges = true
	} else {
		// see if there's a new target release (try to deploy latest if possible)
		// TODO: check if autorelease is enabled for this target..
		newTarget := firstDeployableRelease
		if newTarget != nil {
			if targetRelease != newTarget {
				var previousTarget string
				if targetRelease != nil {
					previousTarget = targetRelease.Name
				}
				hasChanges = true
				logger.Info("Setting new target release", "target", newTarget.Name, "previousTarget", previousTarget)
			}
			targetRelease = newTarget
		}
	}

	// TODO: don't deploy additional builds when outside of schedule
	// TODO: when there are canaries, compute remaining percentage here
	desiredInstances := at.DesiredInstances()
	targetTrafficPercentage := targetRelease.Spec.TrafficPercentage

	increment := float32(rampIncrement)
	if !at.NeedsService() {
		// for apps without services, there's no need to slowly ramp
		increment = 1.0
	}

	if desiredInstances == 0 {
		logger.Info("Scaling target to 0 instances", "release", targetRelease.Name)
		// technically nothing should be getting traffic.. but if it's not set to 100% istio will reject config
		targetTrafficPercentage = 100
		if targetRelease.Spec.NumDesired != 0 {
			hasChanges = true
		}
		targetRelease.Spec.NumDesired = desiredInstances
		// flip to active
		activeRelease = targetRelease
	} else if targetRelease == activeRelease {
		targetTrafficPercentage = 100
		targetRelease.Spec.NumDesired = desiredInstances
	} else {
		// increase by up to rampIncrement
		maxIncrement := int32(float32(desiredInstances) * increment)
		if maxIncrement < 1 {
			maxIncrement = 1
		}
		// if earlier instances aren't available, don't ramp new instances.
		// it's likely something is wrong
		targetInstances := targetRelease.Status.NumAvailable + maxIncrement

		if targetInstances > desiredInstances {
			targetInstances = desiredInstances
		}
		if targetRelease.Spec.NumDesired < targetInstances {
			logger.Info("Increasing pods", "release", targetRelease.Name,
				"numDesired", targetRelease.Spec.NumDesired, "newNumDesired", targetInstances)
			targetRelease.Spec.NumDesired = targetInstances
			hasChanges = true
		}

		ratioDeployed := float32(targetRelease.Status.NumAvailable) / float32(desiredInstances)
		if ratioDeployed > 1 {
			ratioDeployed = 1
		}
		targetTrafficPercentage = int32(ratioDeployed * 100)

		// should not increment more than ramp increment% at a time
		rampPercentage := int32(100 * increment)
		if targetTrafficPercentage-targetRelease.Spec.TrafficPercentage > rampPercentage {
			targetTrafficPercentage = targetRelease.Spec.TrafficPercentage + rampPercentage
		}

		if targetRelease.Spec.TrafficPercentage == 100 {
			// traffic already at 100%, update active roles and we are done
			activeRelease = targetRelease
			logger.Info("Target fully deployed, marking as active", "release", targetRelease.Name)
			hasChanges = true
		} else {
			// not fully ramped yet, check again
			res = &ctrl.Result{
				RequeueAfter: at.Spec.Probes.GetReadinessTimeout(),
			}
		}
	}

	// now update state on releases
	// all traffic percentage so far, active should get what remains
	var totalTraffic int32
	for _, ar := range releases {
		if ar == activeRelease {
			if ar.Spec.Role != v1alpha1.ReleaseRoleActive {
				logger.Info("setting release role to active", "release", ar.Name, "oldRole", ar.Spec.Role)
			}
			ar.Spec.Role = v1alpha1.ReleaseRoleActive
			if ar == targetRelease {
				ar.Spec.TrafficPercentage = targetTrafficPercentage
			} else {
				// scale down pods as a proportion of traffic
				instanceCount := int32(float32(desiredInstances) * float32(ar.Spec.TrafficPercentage) / 100)
				if desiredInstances > 0 && instanceCount < 1 && ar.Spec.TrafficPercentage != 0 {
					instanceCount = 1
				}

				logger.Info("scaling down activeRelease instance", "count", instanceCount)
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
		} else if ar.Spec.Role == v1alpha1.ReleaseRoleBad {
			ar.Spec.TrafficPercentage = 0
			ar.Spec.NumDesired = 0
		} else {
			prevRole := ar.Spec.Role
			// TODO: update traffic for canaries
			ar.Spec.Role = v1alpha1.ReleaseRoleNone
			ar.Spec.TrafficPercentage = 0

			// ramp down to zero if traffic has been shifted away for awhile
			timeSinceChange := time.Now().Sub(ar.Status.StateChangedAt.Time)
			if prevRole != v1alpha1.ReleaseRoleActive && timeSinceChange > at.Spec.Probes.GetReadinessTimeout() {
				if ar.Spec.NumDesired != 0 {
					logger.Info("Scaling down instances to zero", "release", ar.Name)
				}
				ar.Spec.NumDesired = 0
			} else {
				// not completely scaled down, reconcile again
				res = &ctrl.Result{
					RequeueAfter: at.Spec.Probes.GetReadinessTimeout(),
				}
			}
		}
		totalTraffic += ar.Spec.TrafficPercentage
	}

	// update target label
	for _, ar := range releases {
		if ar == targetRelease {
			ar.Labels[resources.TargetReleaseLabel] = "1"
		} else {
			delete(ar.Labels, resources.TargetReleaseLabel)
		}
	}

	// if we have over 100%, then lower target until we are within threshold
	if totalTraffic != 100 {
		overage := totalTraffic - 100
		targetRelease.Spec.TrafficPercentage -= overage
	}

	at.Status.ActiveRelease = activeRelease.Name
	at.Status.TargetRelease = targetRelease.Name
	if hasChanges || at.Status.DeployUpdatedAt.IsZero() {
		at.Status.DeployUpdatedAt = metav1.Now()
	}

	// configure status
	if at.Spec.DeployMode == v1alpha1.DeployHalt {
		at.Status.Phase = v1alpha1.AppTargetPhaseHalted
	} else if activeRelease != targetRelease || targetRelease.Status.NumAvailable < targetRelease.Spec.NumDesired {
		at.Status.Phase = v1alpha1.AppTargetPhaseDeploying
	} else {
		at.Status.Phase = v1alpha1.AppTargetPhaseRunning
	}

	// only update app target status when it's not in the middle of a deployment
	if !activeRelease.CreationTimestamp.IsZero() && activeRelease == targetRelease {
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
func (r *DeploymentReconciler) reconcileAutoScaler(ctx context.Context, at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) error {
	// find active release and target
	var activeRelease, targetRelease *v1alpha1.AppRelease
	for _, ar := range releases {
		if ar.Spec.Role == v1alpha1.ReleaseRoleActive {
			activeRelease = ar
		} else if ar.Spec.Role == v1alpha1.ReleaseRoleTarget {
			targetRelease = ar
		}
	}

	// only autoscale when not in the middle of a deploy
	needsScaler := at.NeedsAutoscaler() && activeRelease != nil && targetRelease == nil

	// find all existing scalers
	scalerList := autoscale.HorizontalPodAutoscalerList{}
	err := r.Client.List(ctx, &scalerList, client.MatchingLabels(labelsForAppTarget(at)))
	if err != nil {
		return err
	}

	// remove all the existing scalers and exit
	if !needsScaler {
		for _, scaler := range scalerList.Items {
			r.Log.Info("Deleting unused autoscaler", "appTarget", at.Name)
			if err = r.Client.Delete(ctx, &scaler); err != nil {
				return err
			}
		}
		return nil
	}

	scaler := newAutoscalerForAppTarget(at, activeRelease)
	// see if any of the existing scalers match the current template
	for _, s := range scalerList.Items {
		if s.Labels[resources.AppReleaseLabel] != scaler.Labels[resources.AppReleaseLabel] {
			// delete the other scalers on older releases
			r.Log.Info("Deleting autoscaler for old releases", "appTarget", at.Name)
			if err := r.Client.Delete(ctx, &s); err != nil {
				return err
			}
		}
	}

	op, err := resources.UpdateResourceWithMerge(r.Client, scaler, at, r.Scheme)
	if err != nil {
		return err
	}
	resources.LogUpdates(r.Log, op, "Updated autoscaler", "appTarget", at.Name)

	// update status
	at.Status.LastScaledAt = scaler.Status.LastScaleTime

	return err
}

func appReleaseForTarget(at *v1alpha1.AppTarget, build *v1alpha1.Build, configMap *corev1.ConfigMap) *v1alpha1.AppRelease {
	labels := labelsForAppTarget(at)
	for k, v := range resources.LabelsForBuild(build) {
		labels[k] = v
	}
	labels[v1alpha1.AppTargetHash] = at.GetHash()

	// generate name hash with both appTargetHash and config
	hashStr := at.GetHash()
	if configMap != nil {
		labels[v1alpha1.ConfigHashLabel] = configMap.Labels[v1alpha1.ConfigHashLabel]
		hashStr += "-" + labels[v1alpha1.ConfigHashLabel]
		hashStr = files.Sha1ChecksumString(hashStr)
	}
	name := fmt.Sprintf("%s-%s-%s", at.Spec.App,
		build.CreationTimestamp.Format("20060102-1504"),
		hashStr[:5])

	ar := &v1alpha1.AppRelease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: at.TargetNamespace(),
			Name:      name,
			Labels:    labels,
		},
		Spec: v1alpha1.AppReleaseSpec{
			App:           at.Spec.App,
			Target:        at.Spec.Target,
			Build:         build.Name,
			Role:          v1alpha1.ReleaseRoleNone,
			AppCommonSpec: at.Spec.AppCommonSpec,
		},
	}
	if configMap != nil {
		ar.Spec.Config = configMap.Name
	}
	return ar
}

func newAutoscalerForAppTarget(at *v1alpha1.AppTarget, ar *v1alpha1.AppRelease) *autoscale.HorizontalPodAutoscaler {
	minReplicas := at.Spec.Scale.Min
	maxReplicas := at.Spec.Scale.Max
	if minReplicas == 0 {
		minReplicas = 1
	}
	if maxReplicas < minReplicas {
		maxReplicas = minReplicas
	}
	var metrics []autoscale.MetricSpec
	if at.Spec.Scale.TargetCPUUtilization > 0 {
		metrics = append(metrics, autoscale.MetricSpec{
			Type: autoscale.ResourceMetricSourceType,
			Resource: &autoscale.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscale.MetricTarget{
					Type:               autoscale.UtilizationMetricType,
					AverageUtilization: &at.Spec.Scale.TargetCPUUtilization,
				},
			},
		})
	}

	labels := labelsForAppTarget(at)
	labels[resources.AppReleaseLabel] = ar.Name
	autoscaler := autoscale.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-scaler", at.Spec.App),
			Namespace: at.TargetNamespace(),
			Labels:    labels,
		},
		Spec: autoscale.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscale.CrossVersionObjectReference{
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
