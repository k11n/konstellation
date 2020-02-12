package resources

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestMergeObject(t *testing.T) {
	src := v1alpha1.AppTargetSpec{
		Ports: []v1alpha1.PortSpec{
			{
				Name: "http",
				Port: 80,
			},
			{
				Name:     "udp",
				Port:     1000,
				Protocol: corev1.ProtocolUDP,
			},
		},
	}
	emptyDest := v1alpha1.AppTargetSpec{}
	MergeObject(&emptyDest, &src)
	if !reflect.DeepEqual(emptyDest, src) {
		t.Fatalf("emptyDest isn't equal to src")
	}

	unchangedDest := src
	unchangedDest.Ports[0].Protocol = corev1.ProtocolTCP
	destCopy := unchangedDest.DeepCopy()
	MergeObject(&unchangedDest, &src)
	if !reflect.DeepEqual(&unchangedDest, destCopy) {
		t.Fatalf("unchangedDest was updated by src")
	}
	if reflect.DeepEqual(&unchangedDest, src) {
		t.Fatalf("unchangedDest should not be equal to src")
	}
}

func TestMergeSlice(t *testing.T) {
	srcPorts := []v1alpha1.PortSpec{
		{
			Name: "http",
			Port: 80,
		},
		{
			Name:     "udp",
			Port:     1000,
			Protocol: corev1.ProtocolUDP,
		},
	}

	// try merging slices
	emptySlice := []v1alpha1.PortSpec{}
	MergeSlice(&emptySlice, &srcPorts)
	if !reflect.DeepEqual(emptySlice, srcPorts) {
		t.Fatalf("could not merge into empty ports")
	}

	// slice that's has certain defaults set
	copy := []v1alpha1.PortSpec{
		{
			Name:     "http",
			Port:     80,
			Protocol: corev1.ProtocolTCP,
		},
		{
			Name: "udp",
			Port: 1000,
		},
	}
	MergeSlice(&copy, &srcPorts)
	if copy[0].Protocol != corev1.ProtocolTCP {
		t.Fatalf("protocol overwritten when it shouldn't have been")
	}
	fmt.Printf("copy[1]: %v\n", copy[1])
	if copy[1].Protocol != corev1.ProtocolUDP {
		t.Fatalf("protocol udp not set when it should have")
	}
}
