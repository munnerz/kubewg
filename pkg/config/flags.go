package config

import (
	"flag"
	"fmt"
)

var (
	WGBinary        string
	DeviceName      string
	PeerName        string
	PrivateKeyFile  string
	UseKernelModule bool
	OS              string
)

func AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&WGBinary, "wg-binary", "wg", "Path to the wireguard 'wg' binary")
	fs.StringVar(&DeviceName, "device-name", "utun9", "Wireguard network interface name")
	fs.StringVar(&PeerName, "peer-name", "", "The name of this wireguard peer")
	fs.StringVar(&PrivateKeyFile, "private-key-file", "/Users/James/go/src/github.com/munnerz/kubewg/privatekey", "Path to a file containing the wireguard private key for this peer")
	fs.BoolVar(&UseKernelModule, "use-kernel-module", false, "If true, the 'ip' command will be used to create a wireguard interface using the Linux kernel driver")
	fs.StringVar(&OS, "os", "darwin", "Host OS, used to determine commands to use to configure wireguard. Currently only darwin is supported.")
}

func Complete() error {
	if PeerName == "" {
		return fmt.Errorf("peer name must be set")
	}
	if PrivateKeyFile == "" {
		return fmt.Errorf("path to private key file not set")
	}
	if WGBinary == "" {
		return fmt.Errorf("wireguard binary path not set")
	}
	if OS != "linux" && UseKernelModule {
		return fmt.Errorf("kernel module is only compatible with 'linux' os")
	}
	return nil
}
