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
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
)

/***/

var (
	NOCANC_VERSION string = "Undefined"
	OptSizeLimit   uint
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
	fs.Var(&config.Settings.Blynk.Readers, "readers", "list of reader mappings")
	fs.Var(&config.Settings.Blynk.Writers, "writers", "list of writer mappings")
	return fs
}

func DownloadFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	fs.UintVar(&config.Settings.DownloadSizeLimit, "download-size-limit", config.Settings.DownloadSizeLimit, "Download size limit")
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
	var pin_to_channel map[uint]string
	var name_to_channel map[string]*socket.ChannelUpdate

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
	if len(config.Settings.Blynk.Readers) > 0 {
		pin_to_channel = make(map[uint]string)
		name_to_channel = make(map[string]*socket.ChannelUpdate)

		sl := socket.NewSubscriptionList(socket.ChannelUpdateEvent)
		if err = conn.Subscribe(sl); err != nil {
			return err
		}

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

				name_to_channel[cu.Name] = cu
			}
		}()

		for _, reader := range config.Settings.Blynk.Readers {
			pin_to_channel[reader.Pin] = reader.Channel

			if err = conn.Put(socket.ChannelUpdateRequestEvent, socket.NewChannelUpdateRequest(reader.Channel, 0xFFFF)); err != nil {
				return err
			}

			client.RegisterDeviceReaderFunction(reader.Pin, func(pin uint, body *blynk.Body) {
				channel_name := pin_to_channel[pin]
				if channel_name != "" {
					cu := name_to_channel[channel_name]
					if cu != nil {
						body.PushString(string(cu.Value))
					}
				}
			})
		}
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
				fmt.Printf("\rProgress: %d%%, %d bytes, %d bps", np.Progress, np.BytesTransferred, 8*np.BytesTransferred/dur)
			}

		default:
			return fmt.Errorf("Unexpected event (eid=%d)", eid)
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

func reboot_cmd(fs *flag.FlagSet) error {
	panic("Unimplemented")
	return nil
}

func version_cmd(fs *flag.FlagSet) error {
	fmt.Printf("nocanc version %s-%s-%s\r\n", NOCANC_VERSION, runtime.GOOS, runtime.GOARCH)
	return nil
}

func help_cmd(fs *flag.FlagSet) error {
	xargs := fs.Args()

	progname := path.Base(os.Args[0])

	if len(xargs) == 0 {
		fmt.Printf("Usage:\r\n")
		for _, c := range Commands {
			fmt.Printf("%s %s\r\n\t- %s\r\n", progname, c.usage, c.help)
		}
	} else {
		if len(xargs) == 1 {
			c := FindCommandMatches(xargs[0])
			if len(c) == 1 {
				fmt.Printf("%s %s\r\n\t- %s\r\n", progname, c[0].usage, c[0].help)
				fmt.Printf("This command takes the following options:\r\n")
				fs2 := c[0].flags(xargs[0])
				fs2.VisitAll(func(f *flag.Flag) {
					fmt.Printf("\t-%s\t%s (default %s)\r\n", f.Name, f.Usage, f.DefValue)
				})
			} else {
				fmt.Printf("Ambiguous command. Did you mean:\r\n")
				for _, v := range c {
					fmt.Printf("%s help %s\r\n", progname, v.command)
				}
			}
		}
	}
	return nil
}

type CommandDescriptor struct {
	command   string
	processor func(*flag.FlagSet) error
	flags     func(string) *flag.FlagSet
	usage     string
	help      string
}

var Commands = []*CommandDescriptor{
	{"blynk", blynk_cmd, BlynkFlagSet, "blynk [options]", "Connect to a blynk server (see https://www.blynk.cc/)"},
	{"download", download_cmd, DownloadFlagSet, "download [options] <filename> <node_id>", "Download firmware from node"},
	{"help", nil, EmptyFlagSet, "help <command>", "Provide help about a command, or general help if no command is specified"},
	{"list-channels", list_channels_cmd, BaseFlagSet, "list-channels [options]", "List all channels"},
	{"list-nodes", list_nodes_cmd, BaseFlagSet, "list-nodes [options]", "List all nodes"},
	{"monitor", monitor_cmd, BaseFlagSet, "monitor [options] <eid1> <eid2> ...", "Monitor selected by eid, or all events if no eid specified"},
	{"publish", publish_cmd, BaseFlagSet, "publish [options] <channel_name> <value>", "Publish <value> to <channel_name>"},
	{"read-channel", read_channel_cmd, BaseFlagSet, "read-channel [options] <channel_name>", "Read the content of a channel"},
	{"reboot", reboot_cmd, BaseFlagSet, "reboot [options] <node_id>", "Reboot node"},
	{"upload", upload_cmd, BaseFlagSet, "upload [options] <filename> <node_id>", "Upload firmware (intel hex file) to node"},
	{"version", version_cmd, EmptyFlagSet, "version", "display the version"},
}

func FindCommandMatches(cmd string) []*CommandDescriptor {
	var matches []*CommandDescriptor

	for _, c := range Commands {
		if strings.HasPrefix(c.command, cmd) {
			matches = append(matches, c)
		} else {
		}
	}
	return matches
}

func main() {

	progname := path.Base(os.Args[0])

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "# %s: Missing command\r\n", progname)
		fmt.Fprintf(os.Stderr, "# type `%s help` for usage\r\n", progname)
		os.Exit(-2)
	}

	err := config.Load()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in configuration file: %s\r\n", err)
		os.Exit(-2)
	}

	command := FindCommandMatches(os.Args[1])
	if len(command) == 0 {
		fmt.Fprintf(os.Stderr, "# Unknown command '%s', type '%s help' for a list of valid commands\r\n", os.Args[1], progname)
		os.Exit(-2)
	}
	if len(command) > 1 {
		fmt.Fprintf(os.Stderr, "# Ambiguous command '%s', type '%s help' for a list of valid commands\r\n", os.Args[1], progname)
		os.Exit(-2)
	}

	var fs *flag.FlagSet

	fs = command[0].flags(command[0].command)
	if err = fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "# %s\r\n", err)
		fmt.Fprintf(os.Stderr, "# type '%s help %s' for a list of valid options\r\n", progname, os.Args[1])
		os.Exit(-2)
	}

	if command[0].processor != nil {
		if err = command[0].processor(fs); err != nil {
			fmt.Fprintf(os.Stderr, "# %s failed: %s\r\n", command[0].command, err)
			os.Exit(-1)
		}
	} else {
		help_cmd(fs)
	}
}
