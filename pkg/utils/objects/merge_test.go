package objects

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
)

type port struct {
	Name     string
	Port     int
	Protocol string
}

type spec struct {
	Ports        []port
	Requirements corev1.ResourceRequirements
}

func TestMergeObject(t *testing.T) {
	src := spec{
		Ports: []port{
			{
				Name: "http",
				Port: 80,
			},
			{
				Name:     "udp",
				Port:     1000,
				Protocol: "udp",
			},
		},
	}
	emptyDest := spec{}
	MergeObject(&emptyDest, &src)
	if !apiequality.Semantic.DeepEqual(emptyDest, src) {
		t.Fatalf("emptyDest isn't equal to src")
	}

	unchangedDest := spec{}
	destCopy := spec{}
	Clone(&unchangedDest, &src)
	unchangedDest.Ports[0].Protocol = "tcp"
	Clone(&destCopy, &unchangedDest)

	// should not have updated dest, copy & dest are the same
	MergeObject(&unchangedDest, &src)
	if !apiequality.Semantic.DeepEqual(&unchangedDest, &destCopy) {
		t.Fatalf("unchangedDest was updated by src")
	}
	if apiequality.Semantic.DeepEqual(&unchangedDest, src) {
		t.Fatalf("unchangedDest should not be equal to src")
	}
}

func TestMergeSlice(t *testing.T) {
	srcPorts := []port{
		{
			Name: "http",
			Port: 80,
		},
		{
			Name:     "udp",
			Port:     1000,
			Protocol: "udp",
		},
	}

	// try merging slices
	emptySlice := []port{}
	MergeSlice(&emptySlice, &srcPorts)
	if !apiequality.Semantic.DeepEqual(emptySlice, srcPorts) {
		t.Fatalf("could not merge into empty ports")
	}

	// slice that's has certain defaults set
	copy := []port{
		{
			Name:     "http",
			Port:     80,
			Protocol: "tcp",
		},
		{
			Name: "udp",
			Port: 1000,
		},
	}
	MergeSlice(&copy, &srcPorts)
	if copy[0].Protocol != "tcp" {
		t.Fatalf("protocol overwritten when it shouldn't have been")
	}
	// fmt.Printf("copy[1]: %v\n", copy[1])
	if copy[1].Protocol != "udp" {
		t.Fatalf("protocol udp not set when it should have")
	}
}

func TestMergeStructWithMap(t *testing.T) {
	src := spec{
		Requirements: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
		},
	}

	dst := spec{
		Requirements: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			},
		},
	}

	MergeObject(&dst, &src)
	assert.EqualValues(t, &src, &dst)
	//if !apiequality.Semantic.DeepEqual(src, dst) {
	//	t.Fatalf("src and dst aren't equal")
	//}
}

// does not overwrite empty value
func TestMergeEmptyValue(t *testing.T) {
	src := port{
		Name: "source",
		Port: 0,
	}

	dst := port{
		Name: "dest",
		Port: 100,
	}

	MergeObject(&dst, &src)
	assert.Equal(t, 100, dst.Port)
}
