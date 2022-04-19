package ovs

import (
	"context"
	"errors"

	consul "arkadiuss.dev/ovs-service-mesh-controller/controllers/consul"
	consulapi "github.com/hashicorp/consul/api"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func GetService(client consulapi.Client, serviceName string) (*consulapi.CatalogService, error) {
	services, _, err := client.Catalog().Service(serviceName, "", &consulapi.QueryOptions{})
	if err != nil {
		return nil, err
	}

	for _, service := range services {
		if service.ServiceName == serviceName {
			return service, nil
		}
	}
	return nil, errors.New("not found")
}

func CreateOVSFlows(serviceName string, ctx context.Context) error {
	var log = logf.FromContext(ctx)

	consulClient, err := consul.GetConsulClient()
	if err != nil {
		log.Error(err, "unable to create consul client")
	}

	service, err := GetService(*consulClient, serviceName)

	log.Info("Building OVS rules", "service", service)
	return nil
}
