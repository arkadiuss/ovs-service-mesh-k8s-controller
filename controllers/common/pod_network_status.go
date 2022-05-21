package common

import (
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

var (
	NetworkStatusAnnotation = "k8s.v1.cni.cncf.io/network-status"
	NetworkaNameAnnotation  = "ovs.servicemesh.arkadiuss.dev/ovs-cni-network-name"
)

type NetworkStatus struct {
	Name string
	IPs  []string
	Mac  string
}

func GetSwitchNetwork(pod *corev1.Pod) (*NetworkStatus, error) {
	var networks []NetworkStatus
	networksJsonString := pod.Annotations[NetworkStatusAnnotation]
	err := json.Unmarshal([]byte(networksJsonString), &networks)
	if err != nil {
		return nil, err
	}

	for _, networkStatus := range networks {
		if networkStatus.Name == pod.Annotations[NetworkaNameAnnotation] {
			fmt.Printf("NETWORKSTARTU %s", networksJsonString)
			return &networkStatus, nil
		}
	}
	return nil, errors.New("couldn't find right network")
}
