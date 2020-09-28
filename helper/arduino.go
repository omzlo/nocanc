package helper

import (
	"encoding/json"
	"fmt"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/socket"
)

type ArduinoPort struct {
	Address       string `json:"address"`
	Label         string `json:"label"`
	BoardName     string `json:"boardName"`
	Protocol      string `json:"protocol"`
	ProtocolLabel string `json:"protocolLabel"`
	Prefs         struct {
	} `json:"prefs"`
	IdentificationPrefs struct {
	} `json:"identificationPrefs"`
}

type ArduinoPortDescriptor struct {
	Port ArduinoPort `json:"port"`
}

type ArduinoDiscoveryNodeList struct {
	EventType string                   `json:"eventType"`
	Ports     []*ArduinoPortDescriptor `json:"ports"`
}

type ArduinoDiscoveryNodeUpdate struct {
	EventType string      `json:"eventType"`
	Port      ArduinoPort `json:"port"`
}

func GenerateArduinoDiscoveryNodeUpdate(node *socket.NodeUpdateEvent) (string, error) {
	if node.State == models.NodeStateConnected || node.State == models.NodeStateUnresponsive {
		port := &ArduinoDiscoveryNodeUpdate{
			Port: createArduinoPort(node),
		}
		if node.State == models.NodeStateConnected {
			port.EventType = "add"
		} else {
			port.EventType = "remove"
		}
		r, err := json.MarshalIndent(port, "", "  ")
		if err != nil {
			return "", err
		}
		return string(r), nil
	}
	return "", nil
}

func GenerateArduinoDiscoveryNodeList(list *socket.NodeListEvent) (string, error) {

	port_list := &ArduinoDiscoveryNodeList{EventType: "list", Ports: make([]*ArduinoPortDescriptor, 0, 8)}

	for _, node := range list.Nodes {
		if node.State == models.NodeStateConnected {
			port := &ArduinoPortDescriptor{
				Port: createArduinoPort(node),
			}
			port_list.Ports = append(port_list.Ports, port)
		}
	}
	r, err := json.MarshalIndent(port_list, "", "  ")
	if err != nil {
		return "", err
	}
	return string(r), nil
}

func createArduinoPort(node *socket.NodeUpdateEvent) ArduinoPort {
	return ArduinoPort{
		Address:       fmt.Sprintf("%d", node.NodeId),
		Label:         fmt.Sprintf("node %d [%s]", node.NodeId, node.Udid),
		BoardName:     "Omzlo CANZERO",
		Protocol:      "nocan",
		ProtocolLabel: "NoCAN Nodes",
	}
}

type ArduinoDiscoveryErrorEvent struct {
	EventType string `json:"eventType"`
	Message   string `json:"message"`
}

func ArduinoDiscoverError(msg string) string {
	err := ArduinoDiscoveryErrorEvent{"error", msg}
	r, _ := json.MarshalIndent(err, "", "  ")
	return string(r)
}
