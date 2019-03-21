package client

import (
	"encoding/json"
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocanc/cmd/config"
	"github.com/omzlo/nocanc/intelhex"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/models/device"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/models/properties"
	"github.com/omzlo/nocand/socket"
)

var DefaultJobManager *JobManager = nil

func StartDefaultJobManager() {
	if DefaultJobManager == nil {
		DefaultJobManager = NewJobManager()
	}
}

func ListNodes() (*socket.NodeList, *Error) {

	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ServiceUnavailable(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.NodeListEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ServiceUnavailable(err)
	}
	if err = conn.Put(socket.NodeListRequestEvent, nil); err != nil {
		return nil, ServiceUnavailable(err)
	}

	value, err := conn.WaitFor(socket.NodeListEvent)

	if err != nil {
		return nil, ServiceUnavailable(err)
	}

	nl := new(socket.NodeList)
	if err = nl.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}
	return nl, nil
}

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
				Port: ArduinoPort{
					Address:       fmt.Sprintf("%d", node.Id),
					Label:         fmt.Sprintf("node %d", node.Id),
					BoardName:     "Omzlo CANZERO",
					Protocol:      "nocan",
					ProtocolLabel: "NoCAN Nodes",
				},
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

func arduinoDiscoveryNodeUpdate(node *socket.NodeUpdate) (string, error) {

	if node.State == models.NodeStateConnected || node.State == models.NodeStateUnresponsive {
		port := &ArduinoDiscoverySyncEvent{
			Port: ArduinoPort{
				Address:       fmt.Sprintf("%d", node.Id),
				Label:         fmt.Sprintf("node %d", node.Id),
				BoardName:     "Omzlo CANZERO",
				Protocol:      "nocan",
				ProtocolLabel: "NoCAN Nodes",
			},
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

func GetNode(nodeId uint) (*socket.NodeUpdate, *Error) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ServiceUnavailable(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.NodeUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ServiceUnavailable(err)
	}
	if err = conn.Put(socket.NodeUpdateRequestEvent, socket.NodeUpdateRequest(nodeId)); err != nil {
		return nil, ServiceUnavailable(err)
	}

	value, err := conn.WaitFor(socket.NodeUpdateEvent)

	if err != nil {
		return nil, ServiceUnavailable(err)
	}

	nu := new(socket.NodeUpdate)
	if err = nu.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}

	if nu.State == models.NodeStateUnknown {
		return nil, NotFound(fmt.Sprintf("Node %d not found", nodeId))
	}
	return nu, nil
}

func ListChannels() (*socket.ChannelList, *Error) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ServiceUnavailable(err)
	}
	defer conn.Close()

	sl := socket.NewSubscriptionList(socket.ChannelListEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ServiceUnavailable(err)
	}
	if err = conn.Put(socket.ChannelListRequestEvent, nil); err != nil {
		return nil, ServiceUnavailable(err)
	}

	value, err := conn.WaitFor(socket.ChannelListEvent)

	if err != nil {
		return nil, ServiceUnavailable(err)
	}

	cl := new(socket.ChannelList)

	if err = cl.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}
	return cl, nil

}

func GetChannel(channelId nocan.ChannelId) (*socket.ChannelUpdate, *Error) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ServiceUnavailable(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.ChannelUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ServiceUnavailable(err)
	}
	if err = conn.Put(socket.ChannelUpdateRequestEvent, socket.NewChannelUpdateRequest("", channelId)); err != nil {
		return nil, ServiceUnavailable(err)
	}

	value, err := conn.WaitFor(socket.ChannelUpdateEvent)

	if err != nil {
		return nil, ServiceUnavailable(err)
	}

	cu := new(socket.ChannelUpdate)
	if err = cu.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}

	if cu.Status == socket.CHANNEL_NOT_FOUND {
		return nil, NotFound(fmt.Sprintf("Node %d not found", channelId))
	}

	return cu, nil

}

func UpdateChannel(channelId nocan.ChannelId, channelName string, channelValue []byte) *Error {
	conn, err := config.DialNocanServer()
	if err != nil {
		return ServiceUnavailable(err)
	}
	defer conn.Close()

	err = conn.Put(socket.ChannelUpdateEvent, socket.NewChannelUpdate(channelName, channelId, socket.CHANNEL_UPDATED, channelValue))
	return ServiceUnavailable(err)
}

func GetPowerStatus() (*device.PowerStatus, *Error) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ServiceUnavailable(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.BusPowerStatusUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ServiceUnavailable(err)
	}
	if err = conn.Put(socket.BusPowerStatusUpdateRequestEvent, nil); err != nil {
		return nil, ServiceUnavailable(err)
	}

	value, err := conn.WaitFor(socket.BusPowerStatusUpdateEvent)

	if err != nil {
		return nil, ServiceUnavailable(err)
	}

	ps := new(device.PowerStatus)
	if err = ps.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}

	return ps, nil
}

func UploadFirmware(nodeId uint, firmware *intelhex.IntelHex, updater JobUpdater) (*Job, *Error) {

	upload_request := socket.NewNodeFirmware(nocan.NodeId(nodeId), false)
	for _, block := range firmware.Blocks {
		if block.Type == intelhex.DataRecord {
			upload_request.AppendBlock(block.Address, block.Data)
		} else {
			clog.Debug("Ignoring record of type %d in hex file", block.Type)
		}
	}

	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ServiceUnavailable(err)
	}

	sl := socket.NewSubscriptionList(socket.NodeFirmwareDownloadEvent, socket.NodeFirmwareProgressEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ServiceUnavailable(err)
	}

	if err = conn.Put(socket.NodeFirmwareUploadEvent, upload_request); err != nil {
		return nil, ServiceUnavailable(err)
	}

	job := DefaultJobManager.NewJob(updater)
	go func() {
		for {
			eid, data, err := conn.Get()

			if err != nil {
				job.Fail(err)
				return
			}

			switch eid {
			case socket.NodeFirmwareProgressEvent:
				var np socket.NodeFirmwareProgress

				if err := np.UnpackValue(data); err != nil {
					job.Fail(err)
					return
				}

				switch np.Progress {
				case socket.ProgressSuccess:
					job.Success()
					return
				case socket.ProgressFailed:
					job.Fail(fmt.Errorf("Upload failed"))
					return
				default:
					job.UpdateProgress(float32(np.Progress))
				}
			default:
				job.Fail(fmt.Errorf("Unexpected event during firmware upload (eid=%d)", eid))
				return
			}

		}
	}()
	return job, nil
}

func GetDeviceInformation() (*device.Info, *Error) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ServiceUnavailable(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.DeviceInformationEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ServiceUnavailable(err)
	}
	if err = conn.Put(socket.DeviceInformationRequestEvent, nil); err != nil {
		return nil, ServiceUnavailable(err)
	}

	value, err := conn.WaitFor(socket.DeviceInformationEvent)

	if err != nil {
		return nil, ServiceUnavailable(err)
	}

	di := new(device.Info)
	if err = di.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}

	return di, nil
}

func GetSystemProperties() (*properties.Properties, *Error) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ServiceUnavailable(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.SystemPropertiesEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ServiceUnavailable(err)
	}
	if err = conn.Put(socket.SystemPropertiesRequestEvent, nil); err != nil {
		return nil, ServiceUnavailable(err)
	}

	value, err := conn.WaitFor(socket.SystemPropertiesEvent)

	if err != nil {
		return nil, ServiceUnavailable(err)
	}

	sp := properties.New()
	if err = sp.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}

	return sp, nil
}
