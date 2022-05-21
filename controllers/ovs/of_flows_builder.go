package ovs

import (
	"context"
	"fmt"
	"net"
	"strings"

	types "k8s.io/apimachinery/pkg/types"

	"arkadiuss.dev/ovs-service-mesh-controller/controllers/common"
	"arkadiuss.dev/ovs-service-mesh-controller/controllers/config"
	consul "arkadiuss.dev/ovs-service-mesh-controller/controllers/consul"
	"github.com/digitalocean/go-openvswitch/ovs"
	consulapi "github.com/hashicorp/consul/api"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func GetService(client consulapi.Client, serviceName string) ([]*consulapi.CatalogService, error) {
	services, _, err := client.Catalog().Service(serviceName, "", &consulapi.QueryOptions{})
	if err != nil {
		return nil, err
	}

	return services, nil
}

func CreateOVSFlows(serviceName string, ctx context.Context, k8sClient client.Client) error {
	var log = logf.FromContext(ctx)

	consulClient, err := consul.GetConsulClient()
	if err != nil {
		log.Error(err, "unable to create consul client")
	}

	services, err := GetService(*consulClient, serviceName)

	for _, service := range services {
		log.Info("Building OVS rules", "service", service, "upstreams", len(service.ServiceProxy.Upstreams))
		for _, upstream := range service.ServiceProxy.Upstreams {
			proxyServices, err := GetService(*consulClient, upstream.DestinationName)
			if err != nil || len(proxyServices) < 1 {
				log.Info("No upstream registered yet!", "upstream", upstream.DestinationName)
				continue
			}

			proxyService := proxyServices[0] // TODO: openvswitch group for load balancing

			log.Info("Building OVS rules for upstream", "src", service.ServiceAddress, "dst", proxyService.ServiceAddress)

			proxyPod := &corev1.Pod{}
			proxyPodName := types.NamespacedName{
				Namespace: "default",
				Name:      proxyService.ServiceID,
			}
			err = k8sClient.Get(ctx, proxyPodName, proxyPod)
			if err != nil {
				log.Info("could not fetch PROXY Pod", "error", err.Error())
				return err
			}

			network, err := common.GetSwitchNetwork(proxyPod)
			if err != nil {
				log.Error(err, "Pod in no network")
				return err
			}

			dstMac, err := net.ParseMAC(network.Mac)
			if err != nil {
				log.Error(err, "couldn't parse mac")
				return err
			}

			rules := make([]*ovs.Flow, 3)
			rules[0] = &ovs.Flow{
				Priority: 60,
				Protocol: ovs.ProtocolTCPv4,
				Matches: []ovs.Match{
					ovs.NetworkDestination(config.GetConfig().VirtualIP),
					ovs.TransportDestinationPort(uint16(upstream.LocalBindPort)),
				},
				Actions: []ovs.Action{
					ovs.ConnectionTracking(fmt.Sprintf("commit,zone=1,nat(dst=%s:%d)", proxyService.ServiceAddress, proxyService.ServicePort)),
					ovs.ModDataLinkDestination(dstMac),
					ovs.Normal(),
				},
			}
			rules[1] = &ovs.Flow{
				Priority: 50,
				Protocol: ovs.ProtocolTCPv4,
				Matches: []ovs.Match{
					ovs.ConnectionTrackingState("-trk"),
				},
				Actions: []ovs.Action{
					ovs.ConnectionTracking("table=0,zone=1,nat"),
				},
			}
			rules[2] = &ovs.Flow{
				Priority: 50,
				Protocol: ovs.ProtocolTCPv4,
				Matches: []ovs.Match{
					ovs.ConnectionTrackingState("+est"),
					ovs.ConnectionTrackingZone(1),
					ovs.NetworkDestination(service.ServiceAddress),
				},
				Actions: []ovs.Action{
					ovs.Normal(),
				},
			}
			fmt.Printf("\n\n\n ---- Network rules for %s -----\n\n\n", service.Address)
			for _, rule := range rules {
				rule1Text, _ := rule.MarshalText()
				ruleText := strings.Replace(string(rule1Text), "idle_timeout=0,", "", 1)
				fmt.Printf("sudo ovs-ofctl add-flow br1 \"%s\"\n", ruleText)
			}
			fmt.Printf("\n\n\n---- Rules end -----\n\n\n")

		}
	}
	return nil
}
