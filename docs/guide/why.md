# Why Konstellation

Kubernetes has become the de-facto standard for running workloads on machines. It's been adopted by tech companies big and small. It's also got a vibrant ecosystem, with many wonderful projects that are built on top of Kubernetes, solving [these](https://github.com/kubernetes/autoscaler) [important](https://istio.io/) [problems](https://github.com/kubernetes-sigs/aws-alb-ingress-controller).

However, the learning curve remains steep for developers. For many, using kubernetes means spending days learning about various components, and copying YAML definitions from Medium articles to make it all work. Even when it's set up, it remains a challenge to operate it: from things like rolling back a bad release, to figuring out how to update components to a new version.

I built Konstellation to lower that barrier of entry, giving you all the tools to manage apps on Kubernetes, as well as the lifecycle of Kubernetes clusters themselves. Konstellation is designed to be as easy to use as Heroku, with a focus on reproducibility and maintainability.

## Beta Software

Konstellation is currently in beta. As with most "beta" software, you should expect bugs to be there and be willing to [report them](https://github.com/k11n/konstellation/issues). I'll do my best at addressing incoming issues as quickly as possible.