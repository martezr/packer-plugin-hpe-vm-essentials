package main

import (
	"fmt"
	"os"

	"github.com/martezr/packer-plugin-hpe-vm-essentials/builder/hpe-vme/clone"
	"github.com/martezr/packer-plugin-hpe-vm-essentials/builder/hpe-vme/iso"
	"github.com/martezr/packer-plugin-hpe-vm-essentials/provisioner/morpheus"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
	"github.com/martezr/packer-plugin-hpe-vm-essentials/version"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("iso", new(iso.Builder))
	pps.RegisterBuilder("clone", new(clone.Builder))
	pps.RegisterProvisioner("morpheus", new(morpheus.Provisioner))

	pps.SetVersion(version.PluginVersion)
	err := pps.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
