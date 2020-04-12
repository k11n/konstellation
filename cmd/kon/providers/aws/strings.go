package aws

const (
	subnetMessage = `By default, Konstellation will host all instances behind internet connected subnets.
You would use security groups and load balancers to control incoming traffic.

If you'd like, Konstellation could also set up a more complex networking structure, using
an additional set of private subnets. The latter setup would be more costly.
`
)
