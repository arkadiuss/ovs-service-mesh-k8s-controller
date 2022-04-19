package ovs

import (
	"context"
	"fmt"
	"net"

	"arkadiuss.dev/ovs-service-mesh-controller/controllers/config"
	consul "arkadiuss.dev/ovs-service-mesh-controller/controllers/consul"
	"github.com/digitalocean/go-openvswitch/ovs"
	consulapi "github.com/hashicorp/consul/api"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func GetService(client consulapi.Client, serviceName string) ([]*consulapi.CatalogService, error) {
	services, _, err := client.Catalog().Service(serviceName, "", &consulapi.QueryOptions{})
	if err != nil {
		return nil, err
	}

	return services, nil
}

func CreateOVSFlows(serviceName string, ctx context.Context) error {
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

			// print(f"""ovs-ofctl add-flow {switch.name} \
			// "priority=50,tcp,in_port={port},ip_dst={VIRTUAL_PROXY_IP},tp_dst={local_bind_port},\
			// action=ct(commit,zone=1,nat(dst={dst_address}:{dst_port})),mod_dl_dst:{dst_host.mac},output:{via_port}"\
			// 		""")
			// 		print(f"""ovs-ofctl add-flow {switch.name} \
			// "priority=50,tcp,in_port={via_port},ct_state=-trk,\
			// action=ct(table=0,zone=1,nat)"\
			// 		""")
			// 		print(f"""ovs-ofctl add-flow s1 \
			// "priority=50,tcp,in_port={via_port},ip_dst={src_host.ip},ct_state=+est,ct_zone=1,\
			// action={port}"\
			// 		""")

			// TODO: get from kubernetes
			dstMac, err := net.ParseMAC("00:11:22:33:44:55")
			if err != nil {
				log.Error(err, "couldn't parse mac")
				return err
			}

			rule1 := &ovs.Flow{
				Priority: 50,
				Protocol: ovs.ProtocolTCPv4,
				Matches: []ovs.Match{
					ovs.NetworkDestination(config.GetConfig().ConsulAddr),
					ovs.TransportDestinationPort(uint16(upstream.LocalBindPort)),
				},
				Actions: []ovs.Action{
					ovs.ConnectionTracking(fmt.Sprintf("commit,zone=1,nat(dst=%s:%d)", proxyService.ServiceAddress, proxyService.ServicePort)),
					ovs.ModDataLinkDestination(dstMac),
					ovs.Normal(), // TODO: specific port
				},
			}
			rule1Text, err := rule1.MarshalText()
			log.Info("rule1", "text", rule1Text)

		}
	}
	return nil
}
