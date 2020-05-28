# Principals

## Why Konstellation

Kubernetes is quickly becoming the de-facto standard for running workloads on machines. It's been embraced by Fortune 500s as well as fast-growing startups. It's got a vibrant ecosystem, with many wonderful projects that are built on top of Kubernetes, solving [these](https://github.com/kubernetes/autoscaler) [important](https://istio.io/) [problems](https://github.com/kubernetes-sigs/aws-alb-ingress-controller).

However, the learning curve remains steep for developers. For many, using kubernetes means spending days learning about various components, and copying YAML definitions from Medium articles to make it all work. Even when it's set up, it remains a challenge to operate it: from things like rolling back a bad release, to figuring out how to update components to a new version. These are outside of scope of Kubernetes itself, and yet are important problems when operating a production cluster.

Given the plethora of components that are out there, you have to become proficient in each one to know when to use it, and how to make it work for your needs. As you can imagine, that's a non-trivial amount of knowledge that can become a nightmare to keep track of. Companies with scale devote internal teams to build tooling around Kubernetes in order to make all that manageable. But those tools primary stay in-house, and each company has to reinvent the wheel.

The inspiration for Konstellation came from having worked at two companies that did exactly that. After deciding to adopt Kubernetes, they spent the subsequent months developing internal tools to make it usable. Having been through this experience twice, I wanted an open source set of tools that can solve these pain points; but none existed.

With that thought, I wrote the first line of code for Konstellation in November 2019. From the onset, my goal is to create a solution as simple to use as Heroku, while running on a robust layer that Kubernetes is known for. Konstellation should give you all of the tools necessary to deploy and operate your apps on Kubernetes.

## Compatibility with Kubernetes

Konstellation provides its functionality as a controller running inside of Kubernetes. It doesn't take away or hide any functionality from standard Kubernetes. You could continue to use standard tools like kubectl, and work with any resources that Kubernetes supports natively.

You may also install any standard Kubernetes components to a Konstellation managed cluster. However, you should avoid installing other service meshes, or anything that uses sidecars to enhance routing. Konstellation already deploys Istio, and is tightly integrated with it. Other service meshes would interfere with this setup.

## Reproducibility and undo

With infrastructure changes in the cloud, it can be easy to create a lot of resources that are inter-dependent, making it difficult to remove. Konstellation automates resources management, tracking all of the resources that it creates. For the cluster and VPC, a `destroy` command would remove everything it allocated.

It should also be easy to replicate a cluster, with no manual steps. Konstellation creates cluster manifests and stores them in Kubernetes itself, making it easy to recreate the same setup.

## Optimized for services, not databases

A application typically involves a combination services and databases. While it's possible to run databases inside of Kubernetes, I prefer to run them externally. This is because:

* Databases benefit from having close to the metal access
* Operating databases is very different from operating services, and there are managed services that solve that problem very well. ([RDS](https://aws.amazon.com/rds/), [ElastiCache](https://aws.amazon.com/elasticache/), [ScyllaCloud](https://www.scylladb.com/product/scylla-cloud/) to name a few)
* Scaling databases is tricky, due to the amount of data on disk. The policies used for scaling homogenous services (increasing # of instances) may not work for DBs.

Konstellation focuses on services (used synonymously as apps), and is designed to allow you to point to externally hosted databases via [Configs](apps.md#Configuration).

## Upgrading software

Upgrading major components on a live cluster can be unpredictable. I've had multiple instances where a seemingly simple software upgrade would proceed to take down the entire production cluster, causing downtime and major headaches.

Konstellation takes a different strategy to upgrading software components. For the components Konstellation installs onto a cluster, they are frozen at the time of initial installation. They will not be changed or upgraded, in order to optimize for stability.

With new Konstellation releases, they will include updates to the dependent components, and each release will be tested with the components working together in concert. Konstellation users should create new clusters periodically, and load all of the apps configs that are previously installed. Then switch the DNS endpoint of your domains to shift traffic over to the new cluster. The old cluster could then be deprecated and shut down.

## Versions

Konstellation uses [SemVer](https://semver.org/), and will adopt the following guarantees:

* releases within the same __major__ version will be API compatible between the cluster and CLI
* releases within the same __minor__ version will only include components with only minor version changes

## Beta software

Konstellation is currently in beta. As with most "beta" software, you should expect bugs to be there and be willing to [report them](https://github.com/k11n/konstellation/issues). I'll do my best at addressing incoming issues as quickly as possible.
