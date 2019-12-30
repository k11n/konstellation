package aws

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/spf13/cast"
)

type EC2InstancePricing struct {
	InstanceType               string
	InstanceSeries             string
	InstanceSize               string
	CurrentGeneration          bool
	Memory                     string
	Storage                    string
	VCPUs                      int
	GPUs                       int
	OperatingSystem            string
	MaxIPs                     int
	EBSThroughput              string
	SupportsEnhancedNetworking bool
	NetworkPerformance         string
	OnDemandPriceUSD           float32
}

var regionMapping = map[string]string{
	"us-east-1":      "US East (N. Virginia)",
	"us-east-2":      "US East (Ohio)",
	"us-west-1":      "US West (N. California)",
	"us-west-2":      "US West (Oregon)",
	"ap-east-1":      "Asia Pacific (Hong Kong)",
	"ap-south-1":     "Asia Pacific (Mumbai)",
	"ap-northeast-1": "Asia Pacific (Tokyo)",
	"ap-northeast-2": "Asia Pacific (Seoul)",
	"ap-northeast-3": "Asia Pacific (Osaka-Local)",
	"ap-southeast-1": "Asia Pacific (Singapore)",
	"ap-southeast-2": "Asia Pacific (Sydney)",
	"ca-central-1":   "Canada (Central)",
	"eu-central-1":   "Europe (Frankfurt)",
	"eu-west-1":      "Europe (Ireland)",
	"eu-west-2":      "Europe (London)",
	"eu-west-3":      "Europe (Paris)",
	"eu-north-1":     "Europe (Stockholm)",
	"me-south-1":     "Middle East (Bahrain)",
	"sa-east-1":      "South America (São Paulo)",
}

func ListEC2Instances(svc *pricing.Pricing, region string, currentOnly bool) (instances []*EC2InstancePricing, err error) {
	if regionMapping[region] == "" {
		err = fmt.Errorf("Unknown region for pricing API: %s", region)
		return
	}

	/**
	Available attributes
	AmazonEC2: volumeType, maxIopsvolume, instance, instanceCapacity10xlarge, locationType, toLocationType, instanceFamily, operatingSystem, clockSpeed, LeaseContractLength, ecu, networkPerformance, instanceCapacity8xlarge, group, maxThroughputvolume, gpuMemory, ebsOptimized, maxVolumeSize, gpu, intelAvxAvailable, processorFeatures, instanceCapacity4xlarge, servicecode, groupDescription, elasticGraphicsType, volumeApiName, processorArchitecture, fromLocation, physicalCores, productFamily, fromLocationType, enhancedNetworkingSupported, intelTurboAvailable, memory, dedicatedEbsThroughput, vcpu, OfferingClass, instanceCapacityLarge, capacitystatus, termType, storage, toLocation, intelAvx2Available, storageMedia, physicalProcessor, provisioned, servicename, PurchaseOption, instancesku, productType, instanceCapacity18xlarge, instanceType, tenancy, usagetype, normalizationSizeFactor, instanceCapacity2xlarge, instanceCapacity16xlarge, maxIopsBurstPerformance, instanceCapacity12xlarge, instanceCapacity32xlarge, instanceCapacityXlarge, licenseModel, currentGeneration, preInstalledSw, transferType, location, instanceCapacity24xlarge, instanceCapacity9xlarge, instanceCapacityMedium, operation, resourceType
	**/
	filters := []*pricing.Filter{
		createPricingFilter("operatingSystem", "Linux"),
		createPricingFilter("productFamily", "Compute Instance"),
		createPricingFilter("capacitystatus", "Used"),
		createPricingFilter("tenancy", "Shared"),
		createPricingFilter("operation", "RunInstances"),
		createPricingFilter("location", regionMapping[region]),
	}
	if currentOnly {
		filters = append(filters, createPricingFilter("currentGeneration", "Yes"))
	}
	input := &pricing.GetProductsInput{
		Filters: filters,
	}
	input.SetMaxResults(100)
	input.SetServiceCode("AmazonEC2")
	minSeriesPrice := map[string]float32{}
	err = svc.GetProductsPages(input, func(res *pricing.GetProductsOutput, lastpage bool) bool {
		for _, entry := range res.PriceList {
			var instance *EC2InstancePricing
			instance, err = parseInstance(entry)
			if err != nil {
				return false
			}
			if instance.OnDemandPriceUSD > 0 {
				instances = append(instances, instance)
				if minSeriesPrice[instance.InstanceSeries] == 0 ||
					instance.OnDemandPriceUSD > minSeriesPrice[instance.InstanceSeries] {
					minSeriesPrice[instance.InstanceSeries] = instance.OnDemandPriceUSD
				}
				// if instance.InstanceType == "r5.2xlarge" {
				// 	utils.PrintJSON(entry)
				// }
			}
		}
		return true
	})
	if err != nil {
		return
	}

	// sort items by cheapest instances in each series
	sort.Slice(instances, func(i, j int) bool {
		a := instances[i]
		b := instances[j]
		if a.InstanceSeries == b.InstanceSeries {
			return a.OnDemandPriceUSD < b.OnDemandPriceUSD
		} else {
			return minSeriesPrice[a.InstanceSeries] < minSeriesPrice[b.InstanceSeries]
		}
	})
	return
}

func createPricingFilter(field, value string) *pricing.Filter {
	f := &pricing.Filter{}
	f.SetType(pricing.FilterTypeTermMatch)
	f.SetField(field)
	f.SetValue(value)
	return f
}

func parseInstance(val aws.JSONValue) (instance *EC2InstancePricing, err error) {
	inst := EC2InstancePricing{}
	product, ok := val["product"].(map[string]interface{})
	if !ok {
		err = fmt.Errorf("product field is not a map: %v", val)
		return
	}
	attr, ok := product["attributes"].(map[string]interface{})
	if !ok {
		err = fmt.Errorf("atributes is not a map: %v", product)
		return
	}

	inst.InstanceType = cast.ToString(attr["instanceType"])
	parts := strings.Split(inst.InstanceType, ".")
	inst.InstanceSeries = parts[0]
	inst.InstanceSize = parts[1]
	inst.CurrentGeneration = attr["currentGeneration"] == "Yes"
	inst.Memory = cast.ToString(attr["memory"])
	inst.Storage = cast.ToString(attr["storage"])
	inst.VCPUs = cast.ToInt(attr["vcpu"])
	inst.GPUs = cast.ToInt(attr["gpu"])
	inst.EBSThroughput = cast.ToString(attr["dedicatedEbsThroughput"])
	inst.SupportsEnhancedNetworking = attr["enhancedNetworkingSupported"] == "Yes"
	inst.NetworkPerformance = cast.ToString(attr["networkPerformance"])

	if terms, ok := val["terms"].(map[string]interface{}); ok {
		if onDemand, ok := terms["OnDemand"].(map[string]interface{}); ok {
			for _, offerItem := range onDemand {
				if offer, ok := offerItem.(map[string]interface{}); ok {
					if pricingDimension, ok := offer["priceDimensions"].(map[string]interface{}); ok {
						for _, pricingItem := range pricingDimension {
							if pricingData, ok := pricingItem.(map[string]interface{}); ok {
								if pricePerUnit, ok := pricingData["pricePerUnit"].(map[string]interface{}); ok {
									inst.OnDemandPriceUSD = cast.ToFloat32(pricePerUnit["USD"])
								}
							}
							break
						}
					}
				}
				break
			}
		}
	}

	instance = &inst
	return
}
