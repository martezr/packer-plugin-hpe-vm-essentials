package iso

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomorpheus/morpheus-go-sdk"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

type StepGenerateHTTPTemplates struct {
	TemplateDirectory string
	HTTPDirectory     string
	Ctx               interpolate.Context
}

type generateHttpTemplateData struct {
	StaticIP      string
	StaticMask    string
	StaticGateway string
	StaticDNS     string
	Name          string
}

func (s *StepGenerateHTTPTemplates) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	//debug := state.Get("debug").(bool)
	instance := state.Get("instance").(*morpheus.Instance)
	ui := state.Get("ui").(packersdk.Ui)

	if s.TemplateDirectory == "" {
		log.Println("No template directory given, skipping")
		return multistep.ActionContinue
	}

	c := state.Get("client").(*morpheus.Client)
	ipPoolResponse, err := c.ListNetworkPoolIPAddresses(instance.Interfaces[0].Network.Pool.ID, &morpheus.Request{
		QueryParams: map[string]string{
			"phrase": instance.Name,
		},
	})
	if err != nil {
		log.Println(err)
	}
	result := ipPoolResponse.Result.(*morpheus.ListNetworkPoolIPAddressesResult)
	ipInfo := (*result.NetworkPoolIps)[0]
	instanceStaticIP := ipInfo.IpAddress
	instanceStaticSubnetMask := ipInfo.SubnetMask
	instanceStaticGateway := ipInfo.GatewayAddress
	instanceStaticDNS := ipInfo.DnsServer

	s.Ctx.Data = &generateHttpTemplateData{
		instanceStaticIP,
		instanceStaticSubnetMask,
		instanceStaticGateway,
		instanceStaticDNS,
		instance.Name,
	}

	// Find every template in the template directory
	for _, tempfile := range find(s.TemplateDirectory, ".pkrtpl") {
		log.Println(tempfile)
		userdata, err := os.ReadFile(tempfile)
		if err != nil {
			err := fmt.Errorf("error reading template file: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		out, err := interpolate.Render(string(userdata), &s.Ctx)
		if err != nil {
			err := fmt.Errorf("error rendering template: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		d1 := []byte(out)
		outfile := strings.Replace(tempfile, ".pkrtpl", "", 1)
		file := filepath.Base(outfile)
		outfile_path := fmt.Sprintf("%s/%s", s.HTTPDirectory, file)
		err = os.WriteFile(outfile_path, d1, 0644)
		if err != nil {
			err := fmt.Errorf("error writing template file: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}

	return multistep.ActionContinue
}

func (s *StepGenerateHTTPTemplates) Cleanup(state multistep.StateBag) {}

func find(root, ext string) []string {
	var a []string
	filepath.WalkDir(root, func(s string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if filepath.Ext(d.Name()) == ext {
			a = append(a, s)
		}
		return nil
	})
	return a
}
