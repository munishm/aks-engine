// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"fmt"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-08-01/network"
	"github.com/Azure/go-autorest/autorest/to"
)

func CreateLoadBalancer(prop *api.Properties, isVMSS bool) LoadBalancerARM {

	loadBalancer := LoadBalancerARM{
		ARMResource: ARMResource{
			APIVersion: "[variables('apiVersionNetwork')]",
			DependsOn: []string{
				"[concat('Microsoft.Network/publicIPAddresses/', variables('masterPublicIPAddressName'))]",
			},
		},
		LoadBalancer: network.LoadBalancer{
			Location: to.StringPtr("[variables('location')]"),
			Name:     to.StringPtr("[variables('masterLbName')]"),
			LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
				BackendAddressPools: &[]network.BackendAddressPool{
					{
						Name: to.StringPtr("[variables('masterLbBackendPoolName')]"),
					},
				},
				FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
					{
						Name: to.StringPtr("[variables('masterLbIPConfigName')]"),
						FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
							PublicIPAddress: &network.PublicIPAddress{
								ID: to.StringPtr("[resourceId('Microsoft.Network/publicIpAddresses',variables('masterPublicIPAddressName'))]"),
							},
						},
					},
				},
				LoadBalancingRules: &[]network.LoadBalancingRule{
					{
						Name: to.StringPtr("LBRuleHTTPS"),
						LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
							FrontendIPConfiguration: &network.SubResource{
								ID: to.StringPtr("[variables('masterLbIPConfigID')]"),
							},
							BackendAddressPool: &network.SubResource{
								ID: to.StringPtr("[concat(variables('masterLbID'), '/backendAddressPools/', variables('masterLbBackendPoolName'))]"),
							},
							Protocol:             network.TransportProtocolTCP,
							FrontendPort:         to.Int32Ptr(443),
							BackendPort:          to.Int32Ptr(443),
							EnableFloatingIP:     to.BoolPtr(false),
							IdleTimeoutInMinutes: to.Int32Ptr(5),
							LoadDistribution:     network.Default,
							Probe: &network.SubResource{
								ID: to.StringPtr("[concat(variables('masterLbID'),'/probes/tcpHTTPSProbe')]"),
							},
						},
					},
				},
				Probes: &[]network.Probe{
					{
						Name: to.StringPtr("tcpHTTPSProbe"),
						ProbePropertiesFormat: &network.ProbePropertiesFormat{
							Protocol:          network.ProbeProtocolTCP,
							Port:              to.Int32Ptr(443),
							IntervalInSeconds: to.Int32Ptr(5),
							NumberOfProbes:    to.Int32Ptr(2),
						},
					},
				},
			},
			Sku: &network.LoadBalancerSku{
				Name: "[variables('loadBalancerSku')]",
			},
			Type: to.StringPtr("Microsoft.Network/loadBalancers"),
		},
	}

	if prop.OrchestratorProfile.KubernetesConfig.LoadBalancerSku == "Standard" {
		udpRule := network.LoadBalancingRule{
			Name: to.StringPtr("LBRuleUDP"),
			LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
				FrontendIPConfiguration: &network.SubResource{
					ID: to.StringPtr("[variables('masterLbIPConfigID')]"),
				},
				BackendAddressPool: &network.SubResource{
					ID: to.StringPtr("[concat(variables('masterLbID'), '/backendAddressPools/', variables('masterLbBackendPoolName'))]"),
				},
				Protocol:             network.TransportProtocolUDP,
				FrontendPort:         to.Int32Ptr(1123),
				BackendPort:          to.Int32Ptr(1123),
				EnableFloatingIP:     to.BoolPtr(false),
				IdleTimeoutInMinutes: to.Int32Ptr(5),
				LoadDistribution:     network.Default,
				Probe: &network.SubResource{
					ID: to.StringPtr("[concat(variables('masterLbID'),'/probes/tcpHTTPSProbe')]"),
				},
			},
		}
		*loadBalancer.LoadBalancer.LoadBalancerPropertiesFormat.LoadBalancingRules = append(*loadBalancer.LoadBalancer.LoadBalancerPropertiesFormat.LoadBalancingRules, udpRule)
	}

	if isVMSS {
		inboundNATPools := []network.InboundNatPool{
			{
				Name: to.StringPtr("[concat('SSH-', variables('masterVMNamePrefix'), 'natpools')]"),
				InboundNatPoolPropertiesFormat: &network.InboundNatPoolPropertiesFormat{
					FrontendIPConfiguration: &network.SubResource{
						ID: to.StringPtr("[variables('masterLbIPConfigID')]"),
					},
					Protocol:               network.TransportProtocolTCP,
					BackendPort:            to.Int32Ptr(22),
					FrontendPortRangeStart: to.Int32Ptr(50001),
					FrontendPortRangeEnd:   to.Int32Ptr(50119),
					EnableFloatingIP:       to.BoolPtr(false),
				},
			},
		}
		loadBalancer.InboundNatPools = &inboundNATPools

	} else {
		var inboundNATRules []network.InboundNatRule
		sshNATPorts := []int32{
			22,
			2201,
			2202,
			2203,
			2204,
		}
		for i := 0; i < prop.MasterProfile.Count; i++ {
			inboundNATRule := network.InboundNatRule{
				Name: to.StringPtr(fmt.Sprintf("[concat('SSH-', variables('masterVMNamePrefix'), %d)]", i)),
				InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
					BackendPort:      to.Int32Ptr(22),
					EnableFloatingIP: to.BoolPtr(false),
					FrontendIPConfiguration: &network.SubResource{
						ID: to.StringPtr("[variables('masterLbIPConfigID')]"),
					},
					FrontendPort: to.Int32Ptr(sshNATPorts[i]),
					Protocol:     network.TransportProtocolTCP,
				},
			}
			inboundNATRules = append(inboundNATRules, inboundNATRule)
		}
		loadBalancer.InboundNatRules = &inboundNATRules
	}

	return loadBalancer
}

func CreateMasterInternalLoadBalancer(cs *api.ContainerService) LoadBalancerARM {
	var dependencies []string
	if cs.Properties.MasterProfile.IsCustomVNET() {
		dependencies = append(dependencies, "[variables('nsgID')]")
	} else {
		dependencies = append(dependencies, "[variables('vnetID')]")
	}

	armResource := ARMResource{
		APIVersion: "[variables('apiVersionNetwork')]",
		DependsOn:  dependencies,
	}
	subnet := "[variables('vnetSubnetID')]"
	if cs.Properties.MasterProfile.IsVirtualMachineScaleSets() {
		subnet = "[variables('vnetSubnetIDMaster')]"
	}

	loadBalancer := network.LoadBalancer{
		Location: to.StringPtr("[variables('location')]"),
		Name:     to.StringPtr("[variables('masterInternalLbName')]"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: to.StringPtr("[variables('masterLbBackendPoolName')]"),
				},
			},
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: to.StringPtr("[variables('masterInternalLbIPConfigName')]"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PrivateIPAddress:          to.StringPtr("[variables('kubernetesAPIServerIP')]"),
						PrivateIPAllocationMethod: network.Static,
						Subnet: &network.Subnet{
							ID: to.StringPtr(subnet),
						},
					},
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{
				{
					Name: to.StringPtr("InternalLBRuleHTTPS"),
					LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr("[concat(variables('masterInternalLbID'), '/backendAddressPools/', variables('masterLbBackendPoolName'))]"),
						},
						BackendPort:      to.Int32Ptr(4443),
						EnableFloatingIP: to.BoolPtr(false),
						FrontendIPConfiguration: &network.SubResource{
							ID: to.StringPtr("[variables('masterInternalLbIPConfigID')]"),
						},
						FrontendPort:         to.Int32Ptr(443),
						IdleTimeoutInMinutes: to.Int32Ptr(5),
						Protocol:             network.TransportProtocolTCP,
						Probe: &network.SubResource{
							ID: to.StringPtr("[concat(variables('masterInternalLbID'),'/probes/tcpHTTPSProbe')]"),
						},
					},
				},
			},
			Probes: &[]network.Probe{
				{
					Name: to.StringPtr("tcpHTTPSProbe"),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						IntervalInSeconds: to.Int32Ptr(5),
						NumberOfProbes:    to.Int32Ptr(2),
						Port:              to.Int32Ptr(4443),
						Protocol:          network.ProbeProtocolTCP,
					},
				},
			},
		},
		Sku: &network.LoadBalancerSku{
			Name: network.LoadBalancerSkuName("[variables('loadBalancerSku')]"),
		},
		Type: to.StringPtr("Microsoft.Network/loadBalancers"),
	}

	if cs.Properties.OrchestratorProfile.KubernetesConfig.LoadBalancerSku == "Standard" {
		udpRule := network.LoadBalancingRule{
			Name: to.StringPtr("LBRuleUDP"),
			LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
				BackendAddressPool: &network.SubResource{
					ID: to.StringPtr("[concat(variables('masterInternalLbID'), '/backendAddressPools/', variables('masterLbBackendPoolName'))]"),
				},
				BackendPort:      to.Int32Ptr(1123),
				EnableFloatingIP: to.BoolPtr(false),
				FrontendIPConfiguration: &network.SubResource{
					ID: to.StringPtr("[variables('masterInternalLbIPConfigID')]"),
				},
				FrontendPort:         to.Int32Ptr(1123),
				IdleTimeoutInMinutes: to.Int32Ptr(5),
				Protocol:             network.TransportProtocolUDP,
				Probe: &network.SubResource{
					ID: to.StringPtr("[concat(variables('masterInternalLbID'),'/probes/tcpHTTPSProbe')]"),
				},
			},
		}
		*loadBalancer.LoadBalancerPropertiesFormat.LoadBalancingRules = append(*loadBalancer.LoadBalancerPropertiesFormat.LoadBalancingRules, udpRule)
	}

	loadBalancerARM := LoadBalancerARM{
		ARMResource:  armResource,
		LoadBalancer: loadBalancer,
	}

	return loadBalancerARM
}
