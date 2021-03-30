module github.com/omzlo/nocanc

go 1.16

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/omzlo/clog v0.0.0-20200929154205-ef979337c74c
	github.com/omzlo/go-sscp v0.0.0-20210205211644-9300fad1816f // indirect
	github.com/omzlo/goblynk v0.0.0-20181217093744-d0a5188b4e25
	github.com/omzlo/gomqtt-mini-client v0.0.0-20210214142652-394ed01c053e
	github.com/omzlo/nocand v0.0.0-20210214152337-b26f05af2c49
)

replace (
	github.com/omzlo/clog => ../clog
	github.com/omzlo/go-sscp => ../go-sscp
	github.com/omzlo/goblynk => ../goblynk
	github.com/omzlo/gomqtt-mini-client => ../gomqtt-mini-client
	github.com/omzlo/nocand => ../nocand
)
