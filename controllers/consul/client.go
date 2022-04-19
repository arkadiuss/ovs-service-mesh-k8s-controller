package consul

import (
	"arkadiuss.dev/ovs-service-mesh-controller/controllers/config"
	consulapi "github.com/hashicorp/consul/api"
)

func GetConsulClient() (*consulapi.Client, error) {
	return consulapi.NewClient(&consulapi.Config{
		Address: config.GetConfig().ConsulAddr,
	})
}
