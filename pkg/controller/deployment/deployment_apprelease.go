package deployment

import (
	"context"
	"fmt"
	"time"

	autoscalev2beta2 "k8s.io/api/autoscaling/v2beta2"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/resources"
	"github.com/davidzhao/konstellation/pkg/utils/objects"
)

func (r *ReconcileDeployment) reconcileAppReleases(at *v1alpha1.AppTarget) (releases []*v1alpha1.AppRelease, res *reconcile.Result, err error) {
	// find last N builds
	builds, err := resources.GetBuildsByImage(r.client, at.Spec.BuildRegistry, at.Spec.BuildImage, 5)
	if err != nil {
		return
	}

	// create new releases if needed
	releases, err = resources.GetAppReleases(r.client, at.Spec.App, at.Spec.Target, 20)
	if err != nil {
		return
	}

	// keep track of builds that we already have a release for, those can be ignored
	existingReleases := map[string]bool{}
	for _, ar := range releases {
		existingReleases[ar.Spec.Build] = true
	}

	// create releases for new builds
	for _, b := range builds {
		if existingReleases[b.Name] {
			continue
		}
		ar := appReleaseForTarget(at, &b)
		releases = append(releases, ar)
	}

	// sort releases and determine traffic and latest
	resources.SortAppReleasesByBuild(releases)

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
		existing := &v1alpha1.AppRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ar.Name,
				Namespace: ar.Namespace,
			},
		}
		_, err = controllerutil.CreateOrUpdate(context.TODO(), r.client, existing, func() error {
			if existing.CreationTimestamp.IsZero() {
				if err := controllerutil.SetControllerReference(at, existing, r.scheme); err != nil {
					return err
				}
			}
			existing.Labels = ar.Labels
			objects.MergeObject(&existing.Spec, &ar.Spec)
			return nil
		})
		if err != nil {
			return
		}
	}
	return
}

/**
 * Determine current releases and flip release switch. TODO: figure out how to signal requeue
 */
func (r *ReconcileDeployment) deployReleases(at *v1alpha1.AppTarget, releases []*v1alpha1.AppRelease) (res *reconcile.Result, err error) {
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
		}
	}

	// first deploy, turn on immediately
	if activeRelease == nil {
		activeRelease = releases[0]
		targetRelease = activeRelease
	} else {
		// TODO: don't deploy additional builds when outside of schedule
		// see if there's a new target release (try to deploy latest if possible)
		targetRelease = resources.FirstAvailableRelease(releases)
		if targetRelease == nil {
			// revert back to active, not ready to deploy something new
			targetRelease = activeRelease
		}
	}

	desiredInstances := at.DesiredInstances()
	if targetRelease == activeRelease {
		activeRelease.Spec.TrafficPercentage = 100
		activeRelease.Spec.NumDesired = desiredInstances
	} else {
		// determine current progress, then next steps
		ratioDeployed := float32(targetRelease.Status.NumDesired) / float32(desiredInstances)
		trafficPercentage := int32(ratioDeployed * 100)
		if targetRelease.Status.NumAvailable < targetRelease.Status.NumDesired {
			// not ready  yet.. requeue and try later
			res = &reconcile.Result{
				RequeueAfter: at.Spec.Probes.GetReadinessTimeout() / 2,
			}
		} else {
			if trafficPercentage >= targetRelease.Spec.TrafficPercentage {
				// increase traffic
				targetRelease.Spec.TrafficPercentage = trafficPercentage
				activeRelease.Spec.TrafficPercentage = 100 - trafficPercentage
				// TODO: for canarying releases, give N% of traffic to them
			}
			if ratioDeployed < 1 {
				// compute next step up, increment by 20% at a time
				podsIncrement := int32(float32(desiredInstances) * 0.2)
				if podsIncrement == 0 {
					podsIncrement = 1
				}
				targetRelease.Spec.NumDesired += podsIncrement
				res = &reconcile.Result{
					RequeueAfter: at.Spec.Probes.GetReadinessTimeout(),
				}
			} else {
				// we are fully switched over, update roles
				activeRelease = targetRelease
			}
		}
	}

	// now update state on releases
	earlierThanActive := false
	for _, ar := range releases {
		if ar == activeRelease {
			ar.Spec.Role = v1alpha1.ReleaseRoleActive
			earlierThanActive = true
		} else if ar == targetRelease {
			ar.Spec.Role = v1alpha1.ReleaseRoleTarget
		} else {
			ar.Spec.Role = v1alpha1.ReleaseRoleNone
			// other releases should run at 0 or minimal
			if earlierThanActive {
				ar.Spec.NumDesired = 0
			} else {
				ar.Spec.NumDesired = 1
			}
		}
	}

	at.Status.ActiveRelease = activeRelease.Name
	at.Status.TargetRelease = targetRelease.Name
	at.Status.DeployUpdatedAt = metav1.Now()
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
			if err = r.client.Delete(ctx, &scaler); err != nil {
				return err
			}
		}
		return nil
	}

	var scaler *autoscalev2beta2.HorizontalPodAutoscaler
	scalerTemplate := newAutoscalerForAppTarget(at, activeRelease)
	// see if any of the existing scalers match the current template
	for _, s := range scalerList.Items {
		if s.Labels[resources.APP_RELEASE_LABEL] == scalerTemplate.Labels[resources.APP_RELEASE_LABEL] {
			scaler = &s
		} else {
			// delete the other ones (there should be only one scaler at any time
			if err := r.client.Delete(ctx, &s); err != nil {
				return err
			}
		}
	}

	if scaler == nil {
		scaler = &autoscalev2beta2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: scalerTemplate.Namespace,
				Name:      scalerTemplate.Name,
			},
		}
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.client, scaler, func() error {
		if scaler.CreationTimestamp.IsZero() {
			err := controllerutil.SetControllerReference(activeRelease, scaler, r.scheme)
			if err != nil {
				return err
			}
		}
		scaler.Labels = scalerTemplate.Labels
		objects.MergeObject(&scaler.Spec, &scalerTemplate.Spec)
		return nil
	})
	if err != nil {
		return err
	}

	// update status
	at.Status.LastScaledAt = scaler.Status.LastScaleTime

	return err
}

func appReleaseForTarget(at *v1alpha1.AppTarget, build *v1alpha1.Build) *v1alpha1.AppRelease {
	labels := labelsForAppTarget(at)
	ar := &v1alpha1.AppRelease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: at.TargetNamespace(),
			Labels:    labels,
			// name will be set later with a helper
		},
		Spec: v1alpha1.AppReleaseSpec{
			App:       at.Spec.App,
			Target:    at.Spec.Target,
			Build:     build.Name,
			Role:      v1alpha1.ReleaseRoleNone,
			Ports:     at.Spec.Ports,
			Command:   at.Spec.Command,
			Args:      at.Spec.Args,
			Env:       at.Spec.Env,
			Resources: at.Spec.Resources,
			Probes:    at.Spec.Probes,
		},
	}
	ar.Name = ar.NameFromBuild(build)
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
	labels[resources.APP_RELEASE_LABEL] = ar.Name
	autoscaler := autoscalev2beta2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-scaler", at.Spec.App),
			Namespace: at.TargetNamespace(),
			Labels:    labels,
		},
		Spec: autoscalev2beta2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalev2beta2.CrossVersionObjectReference{
				Kind: "ReplicaSet",
				Name: ar.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
		},
	}
	return &autoscaler
}
