package main

import (
	"flag"
	"fmt"
	"github.com/omzlo/goblynk"
	"github.com/omzlo/nocanc/cmd/config"
	"github.com/omzlo/nocanc/intelhex"
	"github.com/omzlo/nocand/models/device"
	"github.com/omzlo/nocand/models/helpers"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/socket"
	"os"
	"path"
	"runtime"
	"strconv"
	"time"
)

/***/

var (
	NOCANC_VERSION string = "Undefined"
)

func EmptyFlagSet(cmd string) *flag.FlagSet {
	return flag.NewFlagSet(cmd, flag.ExitOnError)
}

func BaseFlagSet(cmd string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.StringVar(&config.Settings.EventServer, "event-server", config.Settings.EventServer, "Address of event server")
	fs.StringVar(&config.Settings.AuthToken, "auth-token", config.Settings.AuthToken, "Authentication key")
	return fs
}

func BlynkFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	fs.StringVar(&config.Settings.Blynk.BlynkServer, "blynk-server", config.Settings.Blynk.BlynkServer, "Address of blynk server")
	fs.StringVar(&config.Settings.Blynk.BlynkToken, "blynk-token", config.Settings.Blynk.BlynkToken, "Blynk authentication token value")
	fs.Var(&config.Settings.Blynk.Notifiers, "notifiers", "List of channels to use for blynk notifications (experimental)")
	fs.Var(&config.Settings.Blynk.Readers, "readers", "list of reader mappings")
	fs.Var(&config.Settings.Blynk.Writers, "writers", "list of writer mappings")
	return fs
}

func DownloadFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	fs.UintVar(&config.Settings.DownloadSizeLimit, "download-size-limit", config.Settings.DownloadSizeLimit, "Download size limit")
	return fs
}

func VersionFlagSet(cmd string) *flag.FlagSet {
	fs := EmptyFlagSet(cmd)
	fs.BoolVar(&config.Settings.CheckForUpdates, "check-for-updates", config.Settings.CheckForUpdates, "Check if a new version of nocanc is available")
	return fs
}

/***/

func monitor_cmd(fs *flag.FlagSet) error {

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

func publish_cmd(fs *flag.FlagSet) error {

	args := fs.Args()

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

func blynk_cmd(fs *flag.FlagSet) error {
	var channel_to_pin map[string]uint
	var channel_notify map[string]bool

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := blynk.NewClient(config.Settings.Blynk.BlynkServer, config.Settings.Blynk.BlynkToken)

	fmt.Fprintf(os.Stderr, "There are %d blynk writers.\r\n", len(config.Settings.Blynk.Writers))
	for _, it_writer := range config.Settings.Blynk.Writers {
		writer := it_writer
		client.RegisterDeviceWriterFunction(writer.Pin, func(pin uint, body blynk.Body) {
			val, ok := body.AsString(0)
			if ok {
				conn.Put(socket.ChannelUpdateEvent, socket.NewChannelUpdate(writer.Channel, 0xFFFF, socket.CHANNEL_UPDATED, []byte(val)))
			}
		})
	}

	fmt.Fprintf(os.Stderr, "There are %d blynk readers.\r\n", len(config.Settings.Blynk.Readers))
	if len(config.Settings.Blynk.Readers) > 0 || len(config.Settings.Blynk.Notifiers) > 0 {
		channel_to_pin = make(map[string]uint)
		channel_notify = make(map[string]bool)

		sl := socket.NewSubscriptionList(socket.ChannelUpdateEvent)
		if err = conn.Subscribe(sl); err != nil {
			return err
		}

		for _, channel := range config.Settings.Blynk.Notifiers {
			channel_notify[channel] = true
		}

		client.OnConnect(func(c uint) error {
			if c == 1 {
				for _, reader := range config.Settings.Blynk.Readers {
					channel_to_pin[reader.Channel] = reader.Pin

					if err = conn.Put(socket.ChannelUpdateRequestEvent, socket.NewChannelUpdateRequest(reader.Channel, 0xFFFF)); err != nil {
						return err
					}
				}

			}
			return nil
		})

		go func() {
			for {
				value, err := conn.WaitFor(socket.ChannelUpdateEvent)

				if err != nil {
					return
				}

				cu := new(socket.ChannelUpdate)

				if err = cu.UnpackValue(value); err != nil {
					return
				}

				if ok := channel_notify[cu.Name]; ok {
					client.Notify(fmt.Sprintf("%s: %s", cu.Name, cu.Value))
				}

				vpin, ok := channel_to_pin[cu.Name]
				if ok {
					client.VirtualWrite(vpin, string(cu.Value))
				}
			}
		}()

	}
	client.Run()
	return nil
}

func list_channels_cmd(fs *flag.FlagSet) error {

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

func list_nodes_cmd(fs *flag.FlagSet) error {

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

func read_channel_cmd(fs *flag.FlagSet) error {
	//fs.BoolVar(&OptOnUpdate, "on-update", false, "wait until channel is updated instead of returning last value immediately")

	args := fs.Args()

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

func upload_cmd(fs *flag.FlagSet) error {

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
				fmt.Printf("\rProgress: %d%%, %d bytes, %d bps.", np.Progress, np.BytesTransferred, 8*np.BytesTransferred/dur)
			}

		default:
			return fmt.Errorf("\nUnexpected event (eid=%d)", eid)
		}

	}
	return nil
}

func download_cmd(fs *flag.FlagSet) error {

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
	download_request.Limit = uint32(config.Settings.DownloadSizeLimit)

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
				fmt.Printf("\nDownload succeeded\n")
			case socket.ProgressFailed:
				fmt.Printf("\nDownload failed\n")
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
					fmt.Printf("Saving block of %d bytes, with offset 0x%x\n", len(block.Data), block.Offset)
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

func reboot_cmd(fs *flag.FlagSet) error {
	panic("Unimplemented")
	return nil
}

func power_cmd(fs *flag.FlagSet) error {
	var expect bool

	xargs := fs.Args()

	if len(xargs) != 1 {
		return fmt.Errorf("Expected one parameter: 'on' or 'off'")
	}

	switch xargs[0] {
	case "on", "1":
		expect = true
	case "off", "0":
		expect = false
	default:
		return fmt.Errorf("Parameter can only have one of the following values: 'on', 'off', '1' or '0'.")
	}

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		return err
	}
	defer conn.Close()

	sl := socket.NewSubscriptionList(socket.BusPowerEvent)
	if err = conn.Subscribe(sl); err != nil {
		return err
	}
	if err = conn.Put(socket.BusPowerEvent, socket.BusPower(expect)); err != nil {
		return err
	}

	value, err := conn.WaitFor(socket.BusPowerEvent)

	if err != nil {
		return err
	}

	var power socket.BusPower
	if err = power.UnpackValue(value); err != nil {
		return err
	}

	if bool(power) == expect {
		fmt.Printf("# Bus power set to: %s\r\n", xargs[0])
		return nil
	}
	return fmt.Errorf("Failed to set bus power to %s", xargs[0])
}

func version_cmd(fs *flag.FlagSet) error {
	fmt.Printf("nocanc version %s-%s-%s\r\n", NOCANC_VERSION, runtime.GOOS, runtime.GOARCH)
	if config.Settings.CheckForUpdates {
		fmt.Printf("\r\nChecking if a new version is available for download:\r\n")
		content, err := helpers.CheckForUpdates("http://omzlo.com/downloads/nocanc.version")
		if err != nil {
			return err
		}
		if content[0] != NOCANC_VERSION {
			fmt.Printf(" - Version %s of nocanc is available for download.\r\n", content[0])
			if len(content) > 1 {
				fmt.Printf(" - Release notes:\r\n%s\r\n", content[1])
			}
		} else {
			fmt.Printf(" - This version of nocand is up-to-date\r\n")
		}
	}
	fmt.Printf("\r\n")
	return nil
}

func help_cmd(fs *flag.FlagSet) error {
	xargs := fs.Args()

	if len(xargs) == 0 {

		fmt.Printf("Usage:\r\n")
		fmt.Println(Commands.Usage())

	} else {
		if len(xargs) == 1 {
			c := Commands.Find(xargs[0])
			if c != nil {
				fmt.Printf("Usage:\r\n")
				fmt.Println(c.Usage())
			} else {
				fmt.Printf("Unknonwn command '%s'.\r\n", xargs[0])
				c := Commands.FuzzyMatch(xargs[0])
				if c != nil {
					fmt.Printf("Did you mean '%s'?\r\n", c.Command)
				}
			}
		} else {
			fmt.Printf("help does not accept more than one parameter.\r\n")
		}
	}
	return nil
}

var Commands = helpers.CommandFlagSetList{
	{"blynk", blynk_cmd, BlynkFlagSet, "blynk [options]", "Connect to a blynk server (see https://www.blynk.cc/)"},
	{"download", download_cmd, DownloadFlagSet, "download [options] <filename> <node_id>", "Download firmware from node"},
	{"help", nil, EmptyFlagSet, "help <command>", "Provide help about a command, or general help if no command is specified"},
	{"list-channels", list_channels_cmd, BaseFlagSet, "list-channels [options]", "List all channels"},
	{"list-nodes", list_nodes_cmd, BaseFlagSet, "list-nodes [options]", "List all nodes"},
	{"monitor", monitor_cmd, BaseFlagSet, "monitor [options] <eid1> <eid2> ...", "Monitor selected by eid, or all events if no eid specified"},
	{"power", power_cmd, BaseFlagSet, "power [options] <on|off>", "power on or off the NoCAN bus"},
	{"publish", publish_cmd, BaseFlagSet, "publish [options] <channel_name> <value>", "Publish <value> to <channel_name>"},
	{"read-channel", read_channel_cmd, BaseFlagSet, "read-channel [options] <channel_name>", "Read the content of a channel"},
	{"reboot", reboot_cmd, BaseFlagSet, "reboot [options] <node_id>", "Reboot node"},
	{"upload", upload_cmd, BaseFlagSet, "upload [options] <filename> <node_id>", "Upload firmware (intel hex file) to node"},
	{"version", version_cmd, VersionFlagSet, "version", "display the version"},
}

func main() {

	command, fs, err := Commands.Parse()

	if err != nil {
		fmt.Fprintf(os.Stderr, "# %s\r\n", err)
		fmt.Fprintf(os.Stderr, "# type `%s help` for usage\r\n", path.Base(os.Args[0]))
		os.Exit(-2)
	}

	err = config.Load()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in configuration file: %s\r\n", err)
		os.Exit(-2)
	}

	if command.Processor == nil {
		help_cmd(fs)
	} else {
		err = command.Processor(fs)

		if err != nil {
			fmt.Fprintf(os.Stderr, "# %s failed: %s\r\n", command.Command, err)
			os.Exit(-1)
		}
	}
}
