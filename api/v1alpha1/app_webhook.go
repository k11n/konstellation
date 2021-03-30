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

package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var applog = logf.Log.WithName("app-resource")

var kclient client.Client

func (a *App) SetupWebhookWithManager(mgr ctrl.Manager) error {
	kclient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(a).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-k11n-dev-v1alpha1-app,mutating=false,failurePolicy=fail,groups=k11n.dev,resources=apps,versions=v1alpha1,name=vapp.kb.io

var _ webhook.Validator = &App{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (a *App) ValidateCreate() error {
	applog.Info("validate create", "name", a.Name)
	return a.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (a *App) ValidateUpdate(old runtime.Object) error {
	applog.Info("validate update", "name", a.Name)
	return a.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (a *App) ValidateDelete() error {
	applog.Info("validate delete", "name", a.Name)
	return nil
}

func (a *App) validate() error {
	// validate ingress
	ports := make(map[string]bool)
	for _, port := range a.Spec.Ports {
		ports[port.Name] = true
	}

	for _, target := range a.Spec.Targets {
		if target.Ingress != nil {
			if len(target.Ingress.Hosts) == 0 {
				return fmt.Errorf("target %s missing ingress hostname", target.Name)
			}
			if target.Ingress.Port == "" {
				return fmt.Errorf("target %s missing ingress port", target.Name)
			}
			if !ports[target.Ingress.Port] {
				return fmt.Errorf("port %s not declared (required by target %s)", target.Ingress.Port, target.Name)
			}
		}
	}

	// validate dependencies
	for _, appReference := range a.Spec.Dependencies {
		dep := &App{}
		err := kclient.Get(context.TODO(), types.NamespacedName{Name: appReference.Name}, dep)
		if err != nil {
			return fmt.Errorf("failed to check dependencies: %v", err)
		}

		// dependency target(s)
		targets := make(map[string]bool)
		for _, target := range dep.Spec.Targets {
			targets[target.Name] = true
		}
		if appReference.Target == "" {
			// match all targets
			for _, target := range a.Spec.Targets {
				if !targets[target.Name] {
					return fmt.Errorf("dependency %s missing target %s", appReference.Name, target.Name)
				}
			}
		} else {
			// target specified
			if !targets[appReference.Target] {
				return fmt.Errorf("dependency %s missing target %s", appReference.Name, appReference.Target)
			}
		}

		// dependency port
		if appReference.Port != "" {
			portFound := false
			for _, port := range dep.Spec.Ports {
				if port.Name == appReference.Port {
					portFound = true
					break
				}
			}
			if !portFound {
				return fmt.Errorf("dependency %s missing port %s", appReference.Name, appReference.Port)
			}
		}
	}

	// validate shared configs
	for _, config := range a.Spec.Configs {
		labels := client.MatchingLabels{
			SharedConfigLabel: config,
		}
		appConfigList := AppConfigList{}
		err := kclient.List(context.TODO(), &appConfigList, labels)
		if err != nil {
			return fmt.Errorf("failed to check shared configs: %v", err)
		}
		if len(appConfigList.Items) == 0 {
			return fmt.Errorf("missing shared config %s", config)
		}
		configTargets := make(map[string]bool)
		for _, item := range appConfigList.Items {
			configTargets[item.Labels[TargetLabel]] = true
		}
		if !configTargets[""] {
			for _, target := range a.Spec.Targets {
				if !configTargets[target.Name] {
					return fmt.Errorf("missing shared config %s for target %s", config, target.Name)
				}
			}
		}
	}

	return nil
}
