package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/goblynk"
	"github.com/omzlo/gomqtt-mini-client"
	"github.com/omzlo/nocanc/client"
	"github.com/omzlo/nocanc/cmd/config"
	"github.com/omzlo/nocanc/intelhex"
	"github.com/omzlo/nocanc/webui"
	"github.com/omzlo/nocand/models/device"
	"github.com/omzlo/nocand/models/helpers"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/socket"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"
)

/***/

var (
	NOCANC_VERSION string = "Undefined"
	dummy          string
	forceFlag      bool = false
)

func EmptyFlagSet(cmd string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.StringVar(&dummy, "config", "", "Alternate configuration file")
	return fs
}

func BaseFlagSet(cmd string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.StringVar(&config.Settings.EventServer, "event-server", config.Settings.EventServer, "Address of event server")
	fs.StringVar(&config.Settings.AuthToken, "auth-token", config.Settings.AuthToken, "Authentication key")
	fs.Var(&config.Settings.LogLevel, "log-level", "Log verbosity level (DEBUGXX, DEBUGX, DEBUG, INFO, WARNING, ERROR or NONE)")
	fs.StringVar(&config.Settings.LogTerminal, "log-terminal", config.Settings.LogTerminal, "Log info on the terminal screen (color, plain, none)")
	fs.Var(config.Settings.LogFile, "log-file", "Name of file where logs are stored. Empty value dissables the log file (default is '').")
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

func MqttFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	fs.StringVar(&config.Settings.Mqtt.MqttServer, "mqtt-server", config.Settings.Mqtt.MqttServer, "URL of mqtt server (e.g. mqtts://user:password@example.com)")
	fs.StringVar(&config.Settings.Mqtt.ClientId, "client-id", config.Settings.Mqtt.ClientId, "MQTT client identifier")
	fs.Var(&config.Settings.Mqtt.Publishers, "publishers", "List of channels to publish to the mqtt server")
	fs.Var(&config.Settings.Mqtt.Subscribers, "subscribers", "List of topics to subscribe from the mqtt server")
	return fs
}

func WebuiFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	fs.StringVar(&config.Settings.Webui.WebServer, "web-server", config.Settings.Webui.WebServer, "Local address of server (e.g. localhost:8080)")
	fs.UintVar(&config.Settings.Webui.Refresh, "refresh", config.Settings.Webui.Refresh, "Refresh rate of web UI in milliseconds (e.g. 5000)")
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
	fs.StringVar(&config.Settings.UpdateUrl, "update-url", config.Settings.UpdateUrl, "URL prefix that will be used to check if updates are available")
	return fs
}

func ReadChannelFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	fs.BoolVar(&config.Settings.OnUpdate, "on-update", false, "Wait until channel is updated instead of returning last value immediately")
	return fs
}

func RebootFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	fs.BoolVar(&forceFlag, "force", false, "Force sending reboot request even if the node does not exist.")
	return fs
}

/***/

func DialNocanServer() (*socket.EventConn, error) {
	clog.Debug("Trying to connect to NoCAN event server '%s'", config.Settings.EventServer)

	conn, err := socket.Dial(config.Settings.EventServer, config.Settings.AuthToken)
	if err != nil {
		clog.Warning("Failed to connect to '%s', %s", config.Settings.EventServer, err)
		return nil, fmt.Errorf("Failed to connect to NoCAN server, %s", err)
	}
	clog.Debug("Connected to NoCAN event server '%s'", config.Settings.EventServer)
	return conn, nil
}

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
	conn, err := DialNocanServer()
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
				clog.Warning("Could not unpack event: %s", err)
			} else {
				fmt.Printf("EVENT\t%s\t#%d\t%s\n", eid, eid, ps)
			}
		case socket.ChannelUpdateEvent:
			var cu socket.ChannelUpdate
			if err := cu.UnpackValue(value); err != nil {
				clog.Warning("Could not unpack event: %s", err)
			} else {
				fmt.Printf("EVENT\t%s\t#%d\t%s\n", eid, eid, cu)
			}

		default:
			fmt.Printf("EVENT\t%s\t#%d\t%s\n", eid, eid, value)
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

	conn, err := DialNocanServer()
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Put(socket.ChannelUpdateEvent, socket.NewChannelUpdate(channelName, 0xFFFF, socket.CHANNEL_UPDATED, []byte(channelValue)))
}

func blynk_cmd(fs *flag.FlagSet) error {
	var channel_to_pin map[string]uint
	var channel_notify map[string]bool

	conn, err := DialNocanServer()
	if err != nil {
		return err
	}
	defer conn.Close()

	client := blynk.NewClient(config.Settings.Blynk.BlynkServer, config.Settings.Blynk.BlynkToken)

	clog.Info("There are %d blynk writers.", len(config.Settings.Blynk.Writers))
	for _, it_writer := range config.Settings.Blynk.Writers {
		writer := it_writer
		client.RegisterDeviceWriterFunction(writer.Pin, func(pin uint, body blynk.Body) {
			val, ok := body.AsString(0)
			if ok {
				conn.Put(socket.ChannelUpdateEvent, socket.NewChannelUpdate(writer.Channel, 0xFFFF, socket.CHANNEL_UPDATED, []byte(val)))
			}
		})
	}

	clog.Info("There are %d blynk readers.", len(config.Settings.Blynk.Readers))
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
	return client.RunEventLoop()
}

type mqtt_mapping struct {
	Target    string
	Transform *template.Template
}

func mqtt_cmd(fs *flag.FlagSet) error {

	/**************************/
	/* Setup NoCAN connection */
	/**************************/

	conn, err := DialNocanServer()
	if err != nil {
		return err
	}
	defer conn.Close()

	sl := socket.NewSubscriptionList(socket.ChannelUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return err
	}

	/*************************/
	/* Setup MQTT connection */
	/*************************/

	mqtt, err := gomqtt_mini_client.NewMqttClient(config.Settings.Mqtt.ClientId, config.Settings.Mqtt.MqttServer)
	if err != nil {
		return err
	}

	/**************************/
	/* Setup MQTT subscribers */
	/**************************/

	if len(config.Settings.Mqtt.Subscribers) > 0 {
		channel_sub := make(map[string]mqtt_mapping)

		// We parse and setup the mapping bewteen MQTT topics and NoCAN channels

		for _, subs := range config.Settings.Mqtt.Subscribers {

			if len(subs.Transform) == 0 {
				subs.Transform = `{{ printf "%s" .Value }}`
			}
			template, err := template.New(subs.Topic).Parse(subs.Transform)
			if err != nil {
				clog.Fatal("Invalid MQTT transformation for topic '%s' subscription, %s", subs.Topic, err)
			}
			channel_sub[subs.Topic] = mqtt_mapping{subs.Channel, template}
			clog.Debug("Mapping MQTT topic '%s' to NoCAN channel '%s' for subscription'", subs.Topic, subs.Channel)
		}

		// SubscribeCallback is the function that gets alled when data is published on a MQTT channel
		// we transfer the data to a NoCAN channel, using channel_sub as a mapping.

		mqtt.SubscribeCallback = func(topic string, value []byte) {
			tv := struct {
				Topic string
				Value []byte
			}{
				topic,
				value,
			}
			if mapping, ok := channel_sub[topic]; ok {
				svalue := new(bytes.Buffer)
				if err = mapping.Transform.Execute(svalue, tv); err != nil {
					clog.Warning("Failed to transform value of topic '%s' for MQTT subscription, %s", tv.Topic)
				} else {
					conn.Put(socket.ChannelUpdateEvent, socket.NewChannelUpdate(mapping.Target, 0xFFFF, socket.CHANNEL_UPDATED, svalue.Bytes()))
					clog.Debug("Dispatched %d byte message for NoCAN channel '%s': %q", svalue.Len(), mapping.Target, svalue.Bytes())
				}
			} else {
				clog.Warning("Received message for MQTT topic '%s', but this topic is not mapped to any NoCAN channel", topic)
			}
		}

		// We make sure our MQTT client is subscribed to the relevant topics
		// We only do this once connected, hence the "OnConnect"
		mqtt.OnConnect = func(client *gomqtt_mini_client.MqttClient) {
			for _, subs := range config.Settings.Mqtt.Subscribers {
				client.Subscribe(subs.Topic)
				clog.Debug("Subscribed to MQTT topic %s", subs.Topic)
			}
		}
	}

	/*************************/
	/* Setup MQTT publishers */
	/*************************/

	if len(config.Settings.Mqtt.Publishers) > 0 {
		channel_pub := make(map[string]mqtt_mapping)

		// We parse the mapping between NoCAN channels and MQTT topics

		for _, pubs := range config.Settings.Mqtt.Publishers {

			if len(pubs.Transform) == 0 {
				pubs.Transform = `{{ printf "%s" .Value }}`
			}
			template, err := template.New(pubs.Channel).Parse(pubs.Transform)
			if err != nil {
				clog.Fatal("Invalide MQTT transformation for channel '%s' publications, %s", pubs.Channel, err)
			}
			channel_pub[pubs.Channel] = mqtt_mapping{pubs.Topic, template}
			clog.Debug("Mapping NoCAN channel '%s' to MQTT topic '%s' for publication", pubs.Channel, pubs.Topic)
		}

		// We run a loop that listens to NoCAN channel updates and then propagates them to MQTT topics
		go func() {
			defer mqtt.Disconnect()

			for {
				value, err := conn.WaitFor(socket.ChannelUpdateEvent)

				if err != nil {
					clog.Error(err.Error())
					return
				}

				if mqtt.Connected() {
					cu := new(socket.ChannelUpdate)

					if err = cu.UnpackValue(value); err != nil {
						clog.Error(err.Error())
						return
					}

					if mapping, ok := channel_pub[cu.Name]; ok {
						value := new(bytes.Buffer)
						if err = mapping.Transform.Execute(value, cu); err != nil {
							clog.Warning("Failed to transform value of channel '%s' for MQTT publication, %s", cu.Name)
						} else {
							if err = mqtt.Publish(mapping.Target, value.Bytes()); err != nil {
								clog.Warning("Failed to publish %d bytes from channel '%s' to topic '%s': %s", len(cu.Value), cu.Name, mapping.Target, err)
							} else {
								clog.Info("Published %d bytes from channel '%s' to topic '%s'", len(cu.Value), cu.Name, mapping.Target)
								clog.Debug("Published value is %q", value.Bytes())
							}
						}
					}
				}
			}
		}()
	}

	return mqtt.RunEventLoop()
}

func list_channels_cmd(fs *flag.FlagSet) error {

	conn, err := DialNocanServer()
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
	clog.Debug("Listing %d channels.", len(cl.Channels))
	fmt.Println(cl)
	return nil
}

func list_nodes_cmd(fs *flag.FlagSet) error {

	nl, err := client.ListNodes()

	if err != nil {
		return err
	}
	clog.Debug("Listing %d nodes.", len(nl.Nodes))
	fmt.Println(nl)
	return nil
}

func arduino_discovery_cmd(fs *flag.FlagSet) error {
	var input string

	for {
		var err error

		fmt.Scanln(&input)

		switch input {
		case "START_SYNC":
			for {
				err = client.ArduinoDiscoverNodesSync(func(json string) {
					fmt.Println(json)
				})
				if err != nil {
					fmt.Println(client.ArduinoDiscoverError(err.Error()))
					time.Sleep(15 * time.Second)
				}
			}
		case "START":
			// do nothing
			continue
		case "STOP":
			return nil
		case "LIST":
			json, err := client.ArduinoDiscoverNodes()
			if err != nil {
				fmt.Println(client.ArduinoDiscoverError(err.Error()))
			} else {
				fmt.Println(json)
			}
		default:
			fmt.Println(client.ArduinoDiscoverError(input + " not supported"))
		}
	}
	return nil
}

func device_info_cmd(fs *flag.FlagSet) error {

	di, err := client.GetDeviceInformation()

	if err != nil {
		return err
	}
	clog.Debug("Fetching device information.")
	fmt.Println(di)
	return nil
}

func read_channel_cmd(fs *flag.FlagSet) error {
	args := fs.Args()

	if len(args) != 1 {
		return fmt.Errorf("read-channel command has one argument, %d were provided", len(args))
	}
	channelName := args[0]

	conn, err := DialNocanServer()
	if err != nil {
		return err
	}
	defer conn.Close()

	sl := socket.NewSubscriptionList(socket.ChannelUpdateEvent)
	if err = conn.Subscribe(sl); err != nil {
		return err
	}

	if !config.Settings.OnUpdate {
		if err = conn.Put(socket.ChannelUpdateRequestEvent, socket.NewChannelUpdateRequest(channelName, 0xFFFF)); err != nil {
			return err
		}
	}

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
		clog.Debug("Channel update ignored: <%s>", cu)
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
		return fmt.Errorf("Expected a numerical node identifier, got '%s' instead.", xargs[1])
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
			clog.Debug("Ignoring record of type %d in hex file %s", block.Type, filename)
		}
	}

	conn, err := DialNocanServer()
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

	conn, err := DialNocanServer()
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
	xargs := fs.Args()
	if len(xargs) != 1 {
		return fmt.Errorf("Expected one parameter: a numerical node identifier.")
	}

	nodeid, err := strconv.Atoi(xargs[0])

	if err != nil {
		return fmt.Errorf("Expected a numerical node identifier, got '%s' instead.", xargs[0])
	}

	// Carefull to use xerr and not err
	xerr := client.RebootNode(nodeid, forceFlag)
	if xerr != nil {
		return xerr
	}
	fmt.Println("OK")
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

	conn, err := DialNocanServer()
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
		content, err := helpers.CheckForUpdates("https://www.omzlo.com/downloads/nocanc.version")
		if err != nil {
			return err
		}
		if content[0] != NOCANC_VERSION {
			var extension string

			fmt.Printf(" - Version %s of nocanc is available for download.\r\n", content[0])
			if len(content) > 1 {
				fmt.Printf(" - Release notes:\r\n%s\r\n", content[1])
			}
			if runtime.GOOS == "windows" {
				extension = "zip"
			} else {
				extension = "tar.gz"
			}
			fmt.Printf(" - Download link: https://www.omzlo.com/downloads/nocanc-%s-%s.%s\r\n", runtime.GOOS, runtime.GOARCH, extension)
		} else {
			fmt.Printf(" - This version of nocanc is up-to-date\r\n")
		}
	}
	fmt.Printf("\r\n")
	return nil
}

func webui_cmd(fs *flag.FlagSet) error {
	if config.Settings.CheckForUpdates {
		go client.UpdateLatestNews("webui", NOCANC_VERSION, runtime.GOOS, runtime.GOARCH)
	}
	client.StartDefaultJobManager()
	return webui.Run(config.Settings.Webui.WebServer, config.Settings.Webui.Refresh)
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
	{"arduino-discovery", arduino_discovery_cmd, BaseFlagSet, "arduino-discovery [options]", "For Arduino IDE"},
	{"blynk", blynk_cmd, BlynkFlagSet, "blynk [options]", "Connect to a blynk server (see https://www.blynk.cc/)"},
	{"device-info", device_info_cmd, BaseFlagSet, "device-info [options]", "Get information about the device/hardware."},
	{"download", download_cmd, DownloadFlagSet, "download [options] <filename> <node_id>", "Download firmware from node"},
	{"help", nil, EmptyFlagSet, "help <command>", "Provide help about a command, or general help if no command is specified"},
	{"list-channels", list_channels_cmd, BaseFlagSet, "list-channels [options]", "List all channels"},
	{"list-nodes", list_nodes_cmd, BaseFlagSet, "list-nodes [options]", "List all nodes"},
	{"monitor", monitor_cmd, BaseFlagSet, "monitor [options] <eid1> <eid2> ...", "Monitor selected by eid, or all events if no eid specified"},
	{"mqtt", mqtt_cmd, MqttFlagSet, "mqtt [options]", "Connect to a mqtt server, translating NoCAN channels to MQTT topics."},
	{"power", power_cmd, BaseFlagSet, "power [options] <on|off>", "power on or off the NoCAN bus"},
	{"publish", publish_cmd, BaseFlagSet, "publish [options] <channel_name> <value>", "Publish <value> to <channel_name>"},
	{"read-channel", read_channel_cmd, ReadChannelFlagSet, "read-channel [options] <channel_name>", "Read the content of a channel"},
	{"reboot", reboot_cmd, RebootFlagSet, "reboot [options] <node_id>", "Reboot node"},
	{"upload", upload_cmd, BaseFlagSet, "upload [options] <filename> <node_id>", "Upload firmware (intel hex file) to node"},
	{"version", version_cmd, VersionFlagSet, "version", "display the version"},
	{"webui", webui_cmd, WebuiFlagSet, "webui", "Run web interface"},
}

func CheckForConfigFlag() (bool, string) {
	for k, opt := range os.Args {
		if opt[0] == '-' {
			opt = opt[1:]
			if opt[0] == '-' {
				opt = opt[1:]
			}
			if opt == "config" {
				if k < len(os.Args)+1 {
					return true, os.Args[k+1]
				}
			}
			if strings.HasPrefix(opt, "config=") {
				return true, strings.TrimPrefix(opt, "config=")
			}
		}
	}
	return false, ""
}

func main() {
	var config_loaded bool
	var err error

	conf_opt, file := CheckForConfigFlag()

	if conf_opt {
		config_loaded, err = config.LoadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in configuration file %s: %s\r\n", file, err)
			os.Exit(-2)
		}
		if config_loaded == false {
			fmt.Fprintf(os.Stderr, "Cloud not load configuration file %s\r\n", file)
			os.Exit(-2)
		}
	} else {
		config_loaded, err = config.LoadDefault()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in configuration file %s: %s\r\n", config.DefaultConfigFile, err)
			os.Exit(-2)
		}
	}

	command, fs, err := Commands.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "# %s\r\n", err)
		fmt.Fprintf(os.Stderr, "# type `%s help` for usage\r\n", path.Base(os.Args[0]))
		os.Exit(-2)
	}

	switch config.Settings.LogTerminal {
	case "plain":
		clog.AddWriter(clog.PlainTerminal)
	case "color":
		clog.AddWriter(clog.ColorTerminal)
	case "none":
		// skip
	default:
		fmt.Fprintf(os.Stderr, "# log-terminal setting must be either 'plain', 'color' or 'none'.\r\n")
		os.Exit(-1)
	}
	clog.SetLogLevel(config.Settings.LogLevel)
	if !config.Settings.LogFile.IsNull() {
		clog.AddWriter(clog.NewFileLogWriter(config.Settings.LogFile.String()))
	}

	if config_loaded {
		clog.Debug("Configuration file '%s' was loaded.", config.DefaultConfigFile)
	} else {
		clog.Debug("Configuration file '%s' was not found.", config.DefaultConfigFile)
	}

	if command.Processor == nil {
		help_cmd(fs)
	} else {
		err = command.Processor(fs)

		if err != nil {
			fmt.Fprintf(os.Stderr, "# 'nocanc %s' failed, %s\r\n", command.Command, err)
		}
	}
	clog.Terminate(0)
}
