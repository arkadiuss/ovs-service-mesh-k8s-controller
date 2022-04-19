package consul

import consulapi "github.com/hashicorp/consul/api"

var consulAddr = "http://localhost:8500" // TODO: config

func GetConsulClient() (*consulapi.Client, error) {
	return consulapi.NewClient(&consulapi.Config{
		Address: consulAddr,
	})
}
