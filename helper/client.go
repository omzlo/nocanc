package helper

import (
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocanc/cmd/config"
	"github.com/omzlo/nocanc/intelhex"
	//"github.com/omzlo/nocand/models"
	//"github.com/omzlo/nocand/models/device"
	"github.com/omzlo/nocand/models/nocan"
	//"github.com/omzlo/nocand/models/properties"
	"github.com/omzlo/nocand/socket"
)

func NewNocanClient() *socket.EventConn {
	clog.Debug("Preparing to connect to NoCAN event server '%s'", config.Settings.EventServer)
	return socket.NewEventConn(config.Settings.EventServer, "nocanc", config.Settings.AuthToken)
}

var DefaultJobManager *JobManager = nil

func StartDefaultJobManager() {
	if DefaultJobManager == nil {
		DefaultJobManager = NewJobManager()
	}
}

/*
func ListNodes() (*socket.NodeListEvent, *ExtendedError) {
	list_nodes := make(chan *socket.NodeListEvent)

	nocan_client := NewNocanClient()
	defer nocan_client.Close()

	nocan_client.OnConnect(func(conn *socket.EventConn) error {
		return conn.Send(socket.NewNodeListRequestEvent())
	})

	nocan_client.OnEvent(socket.NodeListEventId, func(conn *socket.EventConn, e socket.Eventer) error {
		list_nodes <- e.(*socket.NodeListEvent)
		return socket.Terminate
	})

	err := nocan_client.DispatchEvents()
	if err != nil && err != socket.Terminate {
		return nil, NewExtendedError(err)
	}

	return <-list_nodes, nil
}

func GetNode(nodeId uint) (*socket.NodeUpdateEvent, *ExtendedError) {
	node_update := make(chan *socket.NodeUpdateEvent)

	nocan_client := NewNocanClient()
	defer nocan_client.Close()

	nocan_client.OnConnect(func(conn *socket.EventConn) error {
		return conn.Send(socket.NewNodeUpdateRequestEvent(nodeId))
	})

	nocan_client.OnEvent(socket.NodeUpdateEventId, func(conn *socket.EventConn, e socket.Eventer) error {
		nu := e.(*NodeUpdateEvent)
		if nu.NodeId == nodeId {
			node_update <- nu
			return socket.Terminate
		}
		return nil
	})

	err := nocan_client.DispatchEvents()
	if err != nil && err != socket.Terminate {
		return nil, ExtendedError(err)
	}

	return <-node_update, nil
}

func ListChannels() (*socket.ChannelListEvent, *ExtendedError) {
	list_channels := make(chan *socket.ChannelListEvent)

	nocan_client := NewNocanClient()
	defer nocan_client.Close()

	nocan_client.OnConnect(func(conn *socket.EventConn) error {
		return conn.Send(socket.NewChannelListRequestEvent())
	})

	nocan_client.OnEvent(socket.NodeListEventId, func(conn *socket.EventConn, e socket.Eventer) error {
		list_nodes <- e.(*socket.NodeListEvent)
		return socket.Terminate
	})

	err := nocan_client.DispatchEvents()
	if err != nil && err != socket.Terminate {
		return nil, ExtendedError(err)
	}

	return <-list_nodes, nil

}

func GetChannel(channelId nocan.ChannelId) (*socket.ChannelUpdate, *ExtendedError) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ExtendError(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.ChannelUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ExtendError(err)
	}
	if err = conn.Put(socket.ChannelUpdateRequestEvent, socket.NewChannelUpdateRequest("", channelId)); err != nil {
		return nil, ExtendError(err)
	}

	value, err := conn.WaitFor(socket.ChannelUpdateEvent)

	if err != nil {
		return nil, ExtendError(err)
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

func UpdateChannel(channelId nocan.ChannelId, channelName string, channelValue []byte) *ExtendedError {
	conn, err := config.DialNocanServer()
	if err != nil {
		return ExtendError(err)
	}
	defer conn.Close()

	err = conn.Put(socket.ChannelUpdateEvent, socket.NewChannelUpdate(channelName, channelId, socket.CHANNEL_UPDATED, channelValue))
	if err != nil {
		return ExtendError(err)
	}
	return nil
}

func GetPowerStatus() (*device.PowerStatus, *ExtendedError) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ExtendError(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.BusPowerStatusUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ExtendError(err)
	}
	if err = conn.Put(socket.BusPowerStatusUpdateRequestEvent, nil); err != nil {
		return nil, ExtendError(err)
	}

	value, err := conn.WaitFor(socket.BusPowerStatusUpdateEvent)

	if err != nil {
		return nil, ExtendError(err)
	}

	ps := new(device.PowerStatus)
	if err = ps.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}

	return ps, nil
}
*/
func UploadFirmware(conn *socket.EventConn, nodeId nocan.NodeId, firmware *intelhex.IntelHex, updater JobUpdater) (*Job, *ExtendedError) {

	upload_request := socket.NewNodeFirmwareEvent(nodeId).ConfigureAsUpload()
	for _, block := range firmware.Blocks {
		if block.Type == intelhex.DataRecord {
			upload_request.AppendBlock(block.Address, block.Data)
		} else {
			clog.Debug("Ignoring record of type %d in hex file", block.Type)
		}
	}

	conn.SendAsync(upload_request, socket.ReturnErrorOrContinue)

	job := DefaultJobManager.NewJob(updater)

	conn.OnEvent(socket.NodeFirmwareProgressEventId, func(conn *socket.EventConn, e socket.Eventer) error {
		np := e.(*socket.NodeFirmwareProgressEvent)

		switch np.Progress {
		case socket.ProgressSuccess:
			job.Success()
		case socket.ProgressFailed:
			job.Fail(fmt.Errorf("Upload failed"))
		default:
			job.UpdateProgress(float32(np.Progress))
		}
		return nil
	})
	return job, nil
}

/*
func GetDeviceInformation() (*device.Information, *ExtendedError) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ExtendError(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.DeviceInformationEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ExtendError(err)
	}
	if err = conn.Put(socket.DeviceInformationRequestEvent, nil); err != nil {
		return nil, ExtendError(err)
	}

	value, err := conn.WaitFor(socket.DeviceInformationEvent)

	if err != nil {
		return nil, ExtendError(err)
	}

	di := new(device.Information)
	if err = di.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}

	return di, nil
}

func GetSystemProperties() (*properties.Properties, *ExtendedError) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ExtendError(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.SystemPropertiesEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ExtendError(err)
	}
	if err = conn.Put(socket.SystemPropertiesRequestEvent, nil); err != nil {
		return nil, ExtendError(err)
	}

	value, err := conn.WaitFor(socket.SystemPropertiesEvent)

	if err != nil {
		return nil, ExtendError(err)
	}

	sp := properties.New()
	if err = sp.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}

	return sp, nil
}

func RebootNode(nodeId int, force bool) *ExtendedError {

	if nodeId > 127 || nodeId == 0 {
		return BadRequest(fmt.Errorf("Node id must be between 1 and 127 included, but %d was provided.", nodeId))
	}

	request := socket.CreateNodeRebootRequest(nocan.NodeId(nodeId), force)

	conn, err := config.DialNocanServer()
	if err != nil {
		return ExtendError(err)
	}
	defer conn.Close()

	if err := conn.Put(socket.NodeRebootRequestEvent, request); err != nil {
		return ExtendError(err)
	}
	if err := conn.GetAck(); err != nil {
		return ExtendError(err).WithInformation(fmt.Sprintf("Node %d", nodeId))
	}
	return nil
}
*/
