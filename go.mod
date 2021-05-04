module github.com/omzlo/nocanc

go 1.16

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/omzlo/clog v0.0.0-20200929154205-ef979337c74c
	github.com/omzlo/goblynk v0.0.0-20181217093744-d0a5188b4e25
	github.com/omzlo/gomqtt-mini-client v0.0.0-20210214142652-394ed01c053e
	github.com/omzlo/nocand v0.0.0-20210214152337-b26f05af2c49
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	golang.org/x/crypto v0.0.0-20210503195802-e9a32991a82e // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20210503173754-0981d6026fa6 // indirect
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace (
	github.com/omzlo/clog => ../clog
	github.com/omzlo/go-sscp => ../go-sscp
	github.com/omzlo/goblynk => ../goblynk
	github.com/omzlo/gomqtt-mini-client => ../gomqtt-mini-client
	github.com/omzlo/nocand => ../nocand
)
