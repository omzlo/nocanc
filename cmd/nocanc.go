package main

import (
	"flag"
	"fmt"
	"github.com/omzlo/goblynk"
	"github.com/omzlo/nocanc/cmd/config"
	"github.com/omzlo/nocanc/intelhex"
	"github.com/omzlo/nocand/models/device"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/socket"
	"os"
	"strconv"
	"strings"
	"time"
)

type BlynkAssoc struct {
	Pin     uint
	Channel string
}

type BlynkMap struct {
	List []BlynkAssoc
}

func (bl *BlynkMap) Set(s string) error {
	v := strings.Split(s, ",")
	for _, item := range v {
		parts := strings.SplitN(item, ":", 2)
		pin, err := strconv.ParseUint(parts[0], 0, 32)
		if err != nil {
			return err
		}
		channel := parts[1]
		bl.List = append(bl.List, BlynkAssoc{uint(pin), channel})
	}
	return nil
}

func (bl *BlynkMap) String() string {
	var s []string

	for _, item := range bl.List {
		s = append(s, fmt.Sprintf("%i:%s", item.Pin, item.Channel))
	}
	return strings.Join(s, ",")
}

var (
	OptSizeLimit    uint
	OptBlynkReaders BlynkMap
	OptBlynkWriters BlynkMap
)

func NewFlagSet(cmd string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.StringVar(&config.Settings.EventServer, "event-server", config.Settings.EventServer, "Address of event server")
	fs.StringVar(&config.Settings.AuthToken, "auth-token", config.Settings.AuthToken, "Authentication key")
	return fs
}

func monitor_cmd(args []string) error {
	fs := NewFlagSet("monitor")

	if err := fs.Parse(args); err != nil {
		return err
	}
	sl := socket.NewSubscriptionList()

	if len(fs.Args()) > 0 {
		for _, arg := range fs.Args() {
			i, err := strconv.ParseUint(arg, 0, 8)
			if err != nil {
				return err
			}
			sl.Add(socket.EventId(i))
		}
	} else {
		for i := 1; i < socket.EventCount; i++ {
			sl.Add(socket.EventId(i))
		}
	}
	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}
	if err = conn.Subscribe(sl); err != nil {
		return err
	}
	defer conn.Close()
	for {
		eid, value, err := conn.Get()
		if err != nil {
			return err
		}
		switch eid {
		case socket.BusPowerStatusUpdateEvent:
			var ps device.PowerStatus
			if err := ps.UnpackValue(value); err != nil {
				fmt.Printf("# Error: %s", err)
			} else {
				fmt.Printf("EVENT\t%s\t#%d\t%s\n", eid, eid, ps)
			}
		default:
			fmt.Printf("EVENT\t%s\t#%d\t%q\n", eid, eid, value)
		}
	}
	return nil
}

func publish_cmd(args []string) error {
	fs := NewFlagSet("publish")

	if err := fs.Parse(args); err != nil {
		return err
	}

	args = fs.Args()

	if len(args) != 2 {
		return fmt.Errorf("publish command has two arguments, %d were provided", len(args))
	}
	channelName := args[0]
	channelValue := args[1]

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Put(socket.ChannelUpdateEvent, socket.NewChannelUpdate(channelName, 0xFFFF, socket.CHANNEL_UPDATED, []byte(channelValue)))
}

func blynk_cmd(args []string) error {
	fs := NewFlagSet("blynk")

	fs.Var(&OptBlynkReaders, "readers", "list of reader mappings")
	fs.Var(&OptBlynkWriters, "writers", "list of writer mappings")

	if err := fs.Parse(args); err != nil {
		return err
	}

	args = fs.Args()

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := blynk.NewClient(blynk.BLYNK_ADDRESS, "0fee55cf5dc54ffd843c9478a5421226")

	for _, writer := range OptBlynkWriters.List {
		client.RegisterDeviceWriterFunction(writer.Pin, func(pin uint, body blynk.Body) {
			val, ok := body.AsString(0)
			if ok {
				conn.Put(socket.ChannelUpdateEvent, socket.NewChannelUpdate(writer.Channel, 0xFFFF, socket.CHANNEL_UPDATED, []byte(val)))
			}
		})
	}
	client.Run()
	return nil
}

func list_channels_cmd(args []string) error {
	fs := NewFlagSet("list-channels")

	if err := fs.Parse(args); err != nil {
		return err
	}

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}
	defer conn.Close()

	sl := socket.NewSubscriptionList(socket.ChannelListEvent)
	if err = conn.Subscribe(sl); err != nil {
		return err
	}
	if err = conn.Put(socket.ChannelListRequestEvent, nil); err != nil {
		return err
	}

	value, err := conn.WaitFor(socket.ChannelListEvent)

	if err != nil {
		return err
	}

	var cl socket.ChannelList

	if err = cl.UnpackValue(value); err != nil {
		return err
	}
	fmt.Println("# Channels:")
	fmt.Println(cl)
	return nil
}

func list_nodes_cmd(args []string) error {
	fs := NewFlagSet("list-nodes")

	if err := fs.Parse(args); err != nil {
		return err
	}

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}
	defer conn.Close()

	sl := socket.NewSubscriptionList(socket.NodeListEvent)
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

	var nl socket.NodeList
	if err = nl.UnpackValue(value); err != nil {
		return err
	}
	fmt.Println("# Nodes:")
	fmt.Println(nl)
	return nil
}

func read_channel_cmd(args []string) error {
	fs := NewFlagSet("read-channel")

	//fs.BoolVar(&OptOnUpdate, "on-update", false, "wait until channel is updated instead of returning last value immediately")

	if err := fs.Parse(args); err != nil {
		return err
	}

	args = fs.Args()

	if len(args) != 1 {
		return fmt.Errorf("read-channel command has one argument, %d were provided", len(args))
	}
	channelName := args[0]

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}
	defer conn.Close()

	sl := socket.NewSubscriptionList(socket.ChannelUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return err
	}
	/*
	   if !OptOnUpdate {
	*/
	if err = conn.Put(socket.ChannelUpdateRequestEvent, socket.NewChannelUpdateRequest(channelName, 0xFFFF)); err != nil {
		return err
	}
	/*
	   }
	*/

	for {
		value, err := conn.WaitFor(socket.ChannelUpdateEvent)

		if err != nil {
			return err
		}

		var cu socket.ChannelUpdate
		if err = cu.UnpackValue(value); err != nil {
			return err
		}
		if cu.Name == channelName {
			fmt.Println(cu)
			break
		}
		fmt.Println("# Channel update ignored: <%s>", cu)
	}
	return nil
}

func upload_cmd(args []string) error {
	fs := NewFlagSet("upload")

	if err := fs.Parse(args); err != nil {
		return err
	}

	xargs := fs.Args()

	if len(xargs) != 2 {
		return fmt.Errorf("Expected two parameters: a file name and a node identifier, but got only %d", len(fs.Args()))
	}

	filename := xargs[0]
	nodeid, err := strconv.Atoi(xargs[1])

	if err != nil {
		return err
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	ihex := intelhex.New()
	if err = ihex.Load(file); err != nil {
		return err
	}

	upload_request := socket.NewNodeFirmware(nocan.NodeId(nodeid), false)
	for _, block := range ihex.Blocks {
		if block.Type == intelhex.DataRecord {
			upload_request.AppendBlock(block.Address, block.Data)
		} else {
			fmt.Printf("Ignoring record of type %d in hex file %s", block.Type, filename)
		}
	}

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}

	sl := socket.NewSubscriptionList(socket.NodeFirmwareDownloadEvent, socket.NodeFirmwareProgressEvent)
	if err = conn.Subscribe(sl); err != nil {
		return err
	}

	if err = conn.Put(socket.NodeFirmwareUploadEvent, upload_request); err != nil {
		return err
	}

	start := time.Now()
	for {
		eid, data, err := conn.Get()

		if err != nil {
			return err
		}

		switch eid {
		case socket.NodeFirmwareProgressEvent:
			var np socket.NodeFirmwareProgress

			if err := np.UnpackValue(data); err != nil {
				return err
			}

			switch np.Progress {
			case socket.ProgressSuccess:
				fmt.Printf("\nDone\n")
				return nil
			case socket.ProgressFailed:
				fmt.Printf("\nFailed\n")
				return fmt.Errorf("Upload failed")
			default:
				dur := uint32(time.Since(start).Seconds())
				if dur == 0 {
					dur = 1
				}
				fmt.Printf("\rProgress: %d%%, %d bytes, %d bps", np.Progress, np.BytesTransferred, 8*np.BytesTransferred/dur)
			}

		default:
			return fmt.Errorf("Unexpected event (eid=%d)", eid)
		}

	}
	return nil
}

func download_cmd(args []string) error {
	fs := NewFlagSet("download")

	fs.UintVar(&OptSizeLimit, "size-limit", (1<<32)-1, "Download size limit")

	if err := fs.Parse(args); err != nil {
		return err
	}

	xargs := fs.Args()

	if len(xargs) != 2 {
		return fmt.Errorf("Expected two parameters: a file name and a node identifier, but got only %d", len(fs.Args()))
	}

	filename := xargs[0]
	nodeid, err := strconv.Atoi(xargs[1])

	if err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}

	sl := socket.NewSubscriptionList(socket.NodeFirmwareDownloadEvent, socket.NodeFirmwareProgressEvent)
	if err = conn.Subscribe(sl); err != nil {
		return err
	}

	download_request := socket.NewNodeFirmware(nocan.NodeId(nodeid), true)
	download_request.Limit = uint32(OptSizeLimit)

	if err = conn.Put(socket.NodeFirmwareDownloadRequestEvent, download_request); err != nil {
		return err
	}

	start := time.Now()
	for {
		eid, data, err := conn.Get()

		if err != nil {
			return err
		}

		switch eid {
		case socket.NodeFirmwareProgressEvent:
			var np socket.NodeFirmwareProgress

			if err := np.UnpackValue(data); err != nil {
				return err
			}

			switch np.Progress {
			case socket.ProgressSuccess:
				fmt.Printf("\nDone\n")
			case socket.ProgressFailed:
				fmt.Printf("\nFailed\n")
				return fmt.Errorf("Download failed")
			default:
				dur := uint32(time.Since(start).Seconds())
				if dur == 0 {
					dur = 1
				}
				fmt.Printf("\rProgress: %d%%, %d bytes, %d bps", np.Progress, np.BytesTransferred, 8*np.BytesTransferred/dur)
			}

		case socket.NodeFirmwareDownloadEvent:
			var nf socket.NodeFirmware
			if err := nf.UnpackValue(data); err != nil {
				return err
			}

			if nf.Id == nocan.NodeId(nodeid) {
				ihex := intelhex.New()
				for _, block := range nf.Code {
					ihex.Add(intelhex.DataRecord, block.Offset, block.Data)
				}
				if err := ihex.Save(file); err != nil {
					return err
				}
				return nil
			}
		default:
			return fmt.Errorf("Unexpected event (eid=%d)", eid)
		}

	}
	return nil
}

func reboot_cmd(args []string) error {
	panic("Unimplemented")
	return nil
}

var help_text = [...]string{
	"monitor [options]                          - Monitor all events",
	"monitor [options] <eid1> <eid2> ...        - Monitor selected event by eid",
	"publish [options] <channel_name> <value>   - Publish <value> to <channel_name>",
	"read-channel [options] <channel_name>      - Read the content of a channel",
	"list-channels [options]                    - List all channels",
	"list-nodes [options]                       - List all nodes",
	"upload [options] <filename> <node_id>      - Upload firmware to node",
	"download [options] <filename> <node_id>    - Download firmware from node",
	"reboot [options] <node_id>                 - Reboot node",
	"",
	"[options] include:",
	"  -event-server <server_address>           - Sever address",
	"                                             (default localhost:4242)",
	"  -auth-key <auth key value>               - Server authentication key",
}

func help() {
	fmt.Printf("Valid commands are 'monitor', 'publish', 'list-channels', 'list-nodes', 'download-firmware'  and 'upload-firmware'\n")
	for _, l := range help_text {
		fmt.Println("  " + l)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("# %s: Missing command\n", os.Args[0])
		help()
		os.Exit(-2)
	}

	config.Load()

	var err error

	switch os.Args[1] {
	case "monitor":
		err = monitor_cmd(os.Args[2:])
	case "publish":
		err = publish_cmd(os.Args[2:])
	case "list-channels":
		err = list_channels_cmd(os.Args[2:])
	case "read-channel":
		err = read_channel_cmd(os.Args[2:])
	case "list-nodes":
		err = list_nodes_cmd(os.Args[2:])
	case "download":
		err = download_cmd(os.Args[2:])
	case "upload":
		err = upload_cmd(os.Args[2:])
	case "reboot":
		err = reboot_cmd(os.Args[2:])
	case "blynk":
		err = blynk_cmd(os.Args[2:])
	default:
		fmt.Printf("%s: Unrecognized command '%s'\n", os.Args[0], os.Args[1])
		help()
		os.Exit(-2)
	}
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
}
