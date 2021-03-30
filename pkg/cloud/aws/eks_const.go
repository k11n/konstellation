package aws

var EKSAvailableVersions = []string{
	"1.19",
	"1.18",
	"1.17",
	"1.16",
}

var EKSAllowedInstanceSeries = map[string]bool{
	"t3":   true,
	"t3a":  true,
	"m5":   true,
	"m5a":  true,
	"c5":   true,
	"r5":   true,
	"r5a":  true,
	"g4dn": true,
	"p2":   true,
	"p3":   true,
	"p3dn": true,
}

// Istio requires 2G of memory for itself, need a bit more breathing room
// realistically the xlarge+ are better instance types

var EKSAllowedInstanceSizes = map[string]bool{
	"medium":   true,
	"large":    true,
	"xlarge":   true,
	"2xlarge":  true,
	"4xlarge":  true,
	"8xlarge":  true,
	"9xlarge":  true,
	"12xlarge": true,
}
