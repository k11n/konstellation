---
home: true
# heroImage: /hero.png
heroText: Konstellation
tagline: An application platform for Kubernetes
actionText: Get Started →
actionLink: /guide/
features:
- title: Simple to use
  details: Konstellation gives you a Heroku-like experience on top of Kubernetes. A fully featured CLI that allows you to manage every aspect of your app deployment. New apps are deployed in minutes with minimal configuration.
- title: Production ready
  details: Built on years of experience with running production Kubernetes clusters. Konstellation provides an integrated stack including load balancing, autoscaling, and release management.
- title: Optimized for AWS
  details: Konstellation is initially optimized to work with AWS EKS. It helps you to set up and manage secure EKS clusters, nodepools, VPCs, and load balancers.
footer: MIT Licensed | Copyright © 2019-2020 David Zhao
---
## App to deployment in minutes

Your time is precious, and it shouldn't be spent on messing with the deployment stack. While Kubernetes provides the scale and efficient resource utilization, it's *raw*, forcing users to think in resources. Konstellation is a layer on top of Kubernetes focused around apps (or services).

```text
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

## One config to rule them all

No one should be copying and pasting resource templates that they don't understand. Konstellation provides high level custom resources and then manages native Kubernetes resources behind the scenes. For external facing apps, specify hostnames and a load balancer will be created automatically.

```yaml
apiVersion: k11n.dev/v1alpha1
kind: App
metadata:
  name: myapp
spec:
  image: registry/myapp
  ports:
    - name: http
      port: 80
  targets:
    - name: production
      scale: {min: 2, max: 10}
      ingress:
        hosts:
          - myapp.mydomain.com
        port: 80
```
