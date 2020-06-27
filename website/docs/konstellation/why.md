---
title: Why Konstellation
---

Kubernetes is quickly becoming the de-facto standard for running workloads on machines. It's been embraced by Fortune 500s as well as startups alike. There's a vibrant ecosystem around it, with many wonderful projects that are built on top of Kubernetes, solving [these](https://github.com/kubernetes/autoscaler) [important](https://istio.io/) [problems](https://prometheus.io/).

However, the learning curve remains steep for developers. For many, using kubernetes means spending weeks and months learning about various components, and copying YAML definitions from [Medium](https://medium.com) posts to make it all work. Even when it's set up, it remains a challenge to operate it: from things like rolling back a bad release, to figuring out how to update components to a new version. These are outside of scope of Kubernetes itself, and yet are important problems when operating a production cluster.

Given the plethora of components that are out there, you have to become proficient in each one to know when to use it, and how to make it work for your needs. As you can imagine, that's a non-trivial amount of knowledge that can become a nightmare to keep track of. Companies with scale devote internal teams to build tooling around Kubernetes in order to make all that manageable. However, those tools primary stay in-house, and every company has to reinvent the wheel.

The inspiration for Konstellation came from having worked at two companies that have done exactly that. After deciding to adopt Kubernetes, they spent more than 6 months developing internal tools to make it practical for developers. Having been through the same experience twice, I wanted an open source set of tools that can solve these pain points; but none existed.

With that thought, I wrote the first line of code for Konstellation in November 2019. From the onset, my goal is to create a solution as simple to use as Heroku, while running on a robust layer that Kubernetes is known for. Konstellation is designed to give you most (if not all) of tools necessary to deploy and operate your apps on Kubernetes.
