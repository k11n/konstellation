---
home: true
# heroImage: /hero.png
heroText: Konstellation
tagline: Kubernetes for humans
actionText: Get Started →
actionLink: /guide/
features:
- title: Apps, not resources
  details: Resources are for machines; Developers focus on apps. A single app.yaml captures everything that's needed to host your app. Konstellation takes care of all the rest.
- title: Code to deploy in minutes
  details: Konstellation manages the entire lifecycles of Kubernetes, giving you a heroku-like experience on your own infrastructure. New apps are deployed in minutes with minimal configuration.
- title: Production ready
  details: Built on years of experience with running production Kubernetes clusters. Konstellation uses the best of breed components to provide an integrated stack
- title: Autoscales
  details: Konstellation scales your app and your cluster automatically depends on traffic. Define your desired resource utilization and the rest is taken care of automatically.
- title: Microservices observability
  details: Built on top of the excellent istio service mesh, Konstellation gives you advanced controls over different versions of your apps. It lets you peek into the traffic flow and troubleshoot issues early on.
- title: Optimized for AWS
  details: Konstellation is initially optimized to work with AWS EKS. It utilizes native ALB load balancers to automatically terminate SSL traffic
footer: MIT Licensed | Copyright © 2019-2020 David Zhao
---
### App to deployment in minutes
```
% kon cluster create
...
% kon app load myapp.yaml
...
% kon config edit myapp
...
% kon app status myapp
Target: development
Hosts: myapp.mydomain.com
Load Balancer: b0d94a8d-istiosystem-konin-a4cf-358886547.us-west-2.elb.amazonaws.com
Scale: 2 min, 10 max

RELEASE                     BUILD                DATE                   PODS    STATUS    TRAFFIC
myapp-20200423-1531-c495    registry/myapp:3     2020-04-23 15:31:40    2/2     released  100%
myapp-20200421-1228-c495    registry/myapp:2     2020-04-21 12:28:11    0       retired   0%
myapp-20200421-1102-b723    registry/myapp:1     2020-04-21 11:02:03    0       retired   0%
```

### One YAML to rule them all
Running apps isn't about copying and pasting templates that you don't understand. Konstellation provides high level custom resources and then in turn translates them into native Kubernetes resources.
```yaml
apiVersion: k11n.dev/v1alpha1
kind: App
metadata:
  name: app2048
spec:
  image: registry/myapp
  ports:
    - name: http
      port: 80
  targets:
    - name: development
      scale: {min: 2, max: 10}
      ingress:
        hosts:
          - myapp.mydomain.com
        port: 80
```