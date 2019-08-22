package client

import (
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

func ListNodes() (*socket.NodeList, *ExtendedError) {

	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ExtendError(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.NodeListEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ExtendError(err)
	}
	if err = conn.Put(socket.NodeListRequestEvent, nil); err != nil {
		return nil, ExtendError(err)
	}

	value, err := conn.WaitFor(socket.NodeListEvent)

	if err != nil {
		return nil, ExtendError(err)
	}

	nl := new(socket.NodeList)
	if err = nl.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}
	return nl, nil
}

func GetNode(nodeId uint) (*socket.NodeUpdate, *ExtendedError) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ExtendError(err)
	}
	defer conn.Close()
	sl := socket.NewSubscriptionList(socket.NodeUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ExtendError(err)
	}
	if err = conn.Put(socket.NodeUpdateRequestEvent, socket.NodeUpdateRequest(nodeId)); err != nil {
		return nil, ExtendError(err)
	}

	value, err := conn.WaitFor(socket.NodeUpdateEvent)

	if err != nil {
		return nil, ExtendError(err)
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

func ListChannels() (*socket.ChannelList, *ExtendedError) {
	conn, err := config.DialNocanServer()
	if err != nil {
		return nil, ExtendError(err)
	}
	defer conn.Close()

	sl := socket.NewSubscriptionList(socket.ChannelListEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ExtendError(err)
	}
	if err = conn.Put(socket.ChannelListRequestEvent, nil); err != nil {
		return nil, ExtendError(err)
	}

	value, err := conn.WaitFor(socket.ChannelListEvent)

	if err != nil {
		return nil, ExtendError(err)
	}

	cl := new(socket.ChannelList)

	if err = cl.UnpackValue(value); err != nil {
		return nil, InternalServerError(err)
	}
	return cl, nil

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

func UploadFirmware(nodeId uint, firmware *intelhex.IntelHex, updater JobUpdater) (*Job, *ExtendedError) {

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
		return nil, ExtendError(err)
	}

	sl := socket.NewSubscriptionList(socket.NodeFirmwareDownloadEvent, socket.NodeFirmwareProgressEvent)
	if err = conn.Subscribe(sl); err != nil {
		return nil, ExtendError(err)
	}

	if err = conn.Put(socket.NodeFirmwareUploadEvent, upload_request); err != nil {
		return nil, ExtendError(err)
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

func GetDeviceInformation() (*device.Info, *ExtendedError) {
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

	di := new(device.Info)
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
