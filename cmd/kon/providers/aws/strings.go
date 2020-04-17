package aws

const (
	topologyMessage = `By default, Konstellation will host all instances behind public subnets.
You would use security groups and load balancers to control incoming traffic.

Konstellation also supports a public + private network topology, where only internet
facing load balancers would be placed in public subnets. This is more secure, but
comes with added complexity and cost (AWS).
`
)
