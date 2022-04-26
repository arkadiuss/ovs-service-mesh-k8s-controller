package consul

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"arkadiuss.dev/ovs-service-mesh-controller/controllers/common"
	"arkadiuss.dev/ovs-service-mesh-controller/controllers/config"
	consulapi "github.com/hashicorp/consul/api"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var consulClient *consulapi.Client

var (
	RegisterAnnotation  = "ovs.servicemesh.arkadiuss.dev/consul-register"
	UpstreamsAnnotation = "ovs.servicemesh.arkadiuss.dev/upstreams"
)

// Inspired by: https://github.com/tczekajlo/kube-consul-register/blob/d710950a4ed16306787ad88516ab63ed3aa0ed8a/controller/pods/controller.go#L350
func PodContainerToConsulService(pod *corev1.Pod, containerStatus corev1.ContainerStatus) (*consulapi.AgentServiceRegistration, error) {
	service := &consulapi.AgentServiceRegistration{}

	service.Name = pod.Labels["app"]
	service.ID = fmt.Sprintf("%s-%s", pod.Name, containerStatus.Name)
	service.Tags = []string{"managed-by:ovs-servicemesh"}
	service.Port = GetContainerPort(pod, containerStatus.Name)

	network, err := common.GetSwitchNetwork(pod)
	if err != nil {
		fmt.Println("No network??") // TODO: handle
	} else {
		service.Address = network.IPs[0]
	}

	upstreamValue, ok := pod.Annotations[UpstreamsAnnotation]
	if ok {
		var upstreams []consulapi.Upstream
		annotationUpstreams := strings.Split(upstreamValue, ",")

		for _, upstreamString := range annotationUpstreams {
			upstream := strings.Split(upstreamString, ":")
			var port int
			port, err := strconv.Atoi(upstream[1])
			if err != nil {
				return nil, err
			}

			upstreams = append(upstreams, consulapi.Upstream{
				LocalBindAddress: config.GetConfig().VirtualIP,
				LocalBindPort:    port,
				DestinationName:  upstream[0],
			})
		}
		service.Proxy = &consulapi.AgentServiceConnectProxyConfig{
			Upstreams: upstreams,
		}
		service.Connect = &consulapi.AgentServiceConnect{
			SidecarService: &consulapi.AgentServiceRegistration{
				Proxy: &consulapi.AgentServiceConnectProxyConfig{
					Upstreams: upstreams,
				},
			},
		}
	}
	return service, nil
}

func GetContainerPort(pod *corev1.Pod, searchContainer string) int {
	for _, container := range pod.Spec.Containers {
		if container.Name == searchContainer {
			if len(container.Ports) > 0 {
				return int(container.Ports[0].ContainerPort)
			}
		}
	}
	return 0
}

func RegisterPodInConsul(pod *corev1.Pod, ctx context.Context) (*[]string, error) {
	var log = logf.FromContext(ctx)

	consulClient, err := GetConsulClient()
	if err != nil {
		log.Error(err, "unable to create consul client")
	}

	if isRegisteredStr, ok := pod.Annotations[RegisterAnnotation]; ok {
		log.Info("Pod is has a tag to register in Consul")
		isRegistered, err := strconv.ParseBool(isRegisteredStr)
		if err != nil || !isRegistered {
			return nil, err
		}

		if pod.Status.Phase == v1.PodRunning {
			var registeredServiceNames []string
			for _, container := range pod.Status.ContainerStatuses {

				if !container.Ready {
					continue
				}

				service, err := PodContainerToConsulService(pod, container)
				if err != nil {
					log.Error(err, "Can't convert POD to Consul's service")
					continue
				}

				log.Info("Container eligible for registration", "service", service)
				err = consulClient.Agent().ServiceRegister(service)
				if err != nil {
					log.Error(err, "Unable to register in consul")
				}

				registeredServiceNames = append(registeredServiceNames, service.Name)
			}
			return &registeredServiceNames, nil
		}
		return nil, nil
	} else {
		log.Info("Pod is not meant to be registered in Consul")
		return nil, nil
	}
}
