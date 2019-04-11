package client

import (
	"encoding/json"
	"fmt"
	"github.com/omzlo/nocanc/cmd/config"
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

type ArduinoDiscoveryListEvent struct {
	EventType string                   `json:"eventType"`
	Ports     []*ArduinoPortDescriptor `json:"ports"`
}

func ArduinoDiscoverNodes() (string, error) {
	nl, err := ListNodes()
	if err != nil {
		return "", err.GoError()
	}

	port_list := &ArduinoDiscoveryListEvent{EventType: "list", Ports: make([]*ArduinoPortDescriptor, 0, 8)}

	for _, node := range nl.Nodes {
		if node.State == models.NodeStateConnected {
			port := &ArduinoPortDescriptor{
				Port: createArduinoPort(node),
			}
			port_list.Ports = append(port_list.Ports, port)
		}
	}
	r, errx := json.MarshalIndent(port_list, "", "  ")
	if errx != nil {
		return "", errx
	}
	return string(r), nil
}

type ArduinoDiscoverySyncEvent struct {
	EventType string      `json:"eventType"`
	Port      ArduinoPort `json:"port"`
}

func createArduinoPort(node *socket.NodeUpdate) ArduinoPort {
	return ArduinoPort{
		Address:       fmt.Sprintf("%d", node.Id),
		Label:         fmt.Sprintf("node %d [%s]", node.Id, node.Udid),
		BoardName:     "Omzlo CANZERO",
		Protocol:      "nocan",
		ProtocolLabel: "NoCAN Nodes",
	}
}

func arduinoDiscoveryNodeUpdate(node *socket.NodeUpdate) (string, error) {

	if node.State == models.NodeStateConnected || node.State == models.NodeStateUnresponsive {
		port := &ArduinoDiscoverySyncEvent{
			Port: createArduinoPort(node),
		}
		if node.State == models.NodeStateConnected {
			port.EventType = "add"
		} else {
			port.EventType = "remove"
		}
		r, errx := json.MarshalIndent(port, "", "  ")
		if errx != nil {
			return "", errx
		}
		return string(r), nil
	}
	return "", nil
}

func ArduinoDiscoverNodesSync(print_cb func(string)) error {

	conn, err := config.DialNocanServer()
	if err != nil {
		return err
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.NodeListEvent, socket.NodeUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return err
	}

	if err = conn.Put(socket.NodeListRequestEvent, nil); err != nil {
		return err
	}

	value, err := conn.WaitFor(socket.NodeListEvent)
	if err != nil {
		return err
	}
	nl := new(socket.NodeList)
	if err = nl.UnpackValue(value); err != nil {
		return err
	}

	for _, node := range nl.Nodes {
		s, err := arduinoDiscoveryNodeUpdate(node)
		if err != nil {
			return err
		}
		print_cb(s)
	}

	for {
		nval, err := conn.WaitFor(socket.NodeUpdateEvent)
		if err != nil {
			return err
		}
		nu := new(socket.NodeUpdate)
		if err = nu.UnpackValue(nval); err != nil {
			return err
		}
		s, err := arduinoDiscoveryNodeUpdate(nu)
		if err != nil {
			return err
		}
		print_cb(s)
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
