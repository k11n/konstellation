# Principals

## Why Konstellation

Kubernetes has become the de-facto standard for running workloads on machines. It's been adopted by tech companies big and small. It's also got a vibrant ecosystem, with many wonderful projects that are built on top of Kubernetes, solving [these](https://github.com/kubernetes/autoscaler) [important](https://istio.io/) [problems](https://github.com/kubernetes-sigs/aws-alb-ingress-controller).

However, the learning curve remains steep for developers. For many, using kubernetes means spending days learning about various components, and copying YAML definitions from Medium articles to make it all work. Even when it's set up, it remains a challenge to operate it: from things like rolling back a bad release, to figuring out how to update components to a new version.

Working with raw Kubernetes is undesirable. Companies that use Kubernetes often build internal tools to make working with Kubernetes more practical. In fact, I've worked at two companies that did exactly that. After deciding to adopt Kubernetes, they spent weeks learning about Kubernetes, and then months developing internal tools to make it usable. Having been through this experience twice, I wanted a set of tools that's open source, and can be used by anyone.

With that thought, I wrote the first line of code for Konstellation in November 2019. I wanted to give users a solution that's as simple as Heroku, while running on a robust infrastructure that you control. Konstellation should give you all of the tools necessary to manage Kubernetes, significantly lowering the barrier of entry.

## Compatibility with Kubernetes

Konstellation is built as a layer on top of Kubernetes, and does not take away or hide any functionality from standard Kubernetes. You could continue to use standard tools like kubectl, and any resources that Kubernetes supports natively.

You may also install any standard Kubernetes components to a Konstellation managed cluster. However, you should avoid installing other service meshes, or anything that uses sidecars to alter the networking stack. Konstellation already deploys Istio, and is tightly integrated with it. Other service meshes would interfere with this setup.

## Reproducibility and undo

With infrastructure changes in the cloud, it can be easy to create a lot of resources that are inter-dependent, making it difficult to remove. Konstellation automates resources management, tracking all of the resources that it creates. For the cluster and VPC, a `destroy` command would remove everything it allocated.

It should also be easy to replicate a cluster, with no manual steps. Konstellation creates cluster manifests and stores them in Kubernetes itself, making it easy to recreate the same setup.

## Apps, not databases

## Upgrading software

Upgrading major components on a live cluster can be unpredictable. I've had multiple instances where a seemingly simple software upgrade would proceed to take down the entire production cluster, causing downtime and major headaches.

Konstellation takes a different strategy to upgrading software components. For the components Konstellation installs onto a cluster, they are frozen at the time of initial installation. They will not be changed or upgraded, in order to optimize for stability.

With new Konstellation releases, they will include updates to the dependent components, and each release will be tested with the components working together in concert. Konstellation users should create new clusters periodically, and load all of the apps configs that are previously installed. Then switch the DNS endpoint of your domains to shift traffic over to the new cluster. The old cluster could then be deprecated and shut down.

## Versions

Konstellation uses [SemVer](https://semver.org/), and will adopt the following guarantees:

* releases within the same __major__ version will be API compatible (work with the CLI)
* releases within the same __minor__ version will only include components with only minor version changes

## Beta software

Konstellation is currently in beta. As with most "beta" software, you should expect bugs to be there and be willing to [report them](https://github.com/k11n/konstellation/issues). I'll do my best at addressing incoming issues as quickly as possible.
