package iso

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/gomorpheus/morpheus-go-sdk"
	"github.com/hashicorp/packer-plugin-sdk/bootcommand"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

type BootConfig struct {
	bootcommand.BootConfig `mapstructure:",squash"`
	// The IP address to use for the HTTP server to serve the `http_directory`.
	HTTPIP string `mapstructure:"http_ip"`
}

type bootCommandTemplateData struct {
	HTTPIP        string
	HTTPPort      int
	StaticIP      string
	StaticMask    string
	StaticGateway string
	StaticDNS     string
	Name          string
}

func (c *BootConfig) Prepare(ctx *interpolate.Context) []error {
	if c.BootWait == 0 {
		c.BootWait = 10 * time.Second
	}

	return c.BootConfig.Prepare(ctx)
}

// This is a definition of a builder step and should implement multistep.Step
type StepTypeBootCommand struct {
	Config *BootConfig
	VMName string
	Ctx    interpolate.Context
}

// Run should execute the purpose of this step
func (s *StepTypeBootCommand) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	debug := state.Get("debug").(bool)
	instance := state.Get("instance").(*morpheus.Instance)
	ui := state.Get("ui").(packersdk.Ui)

	if s.Config.BootCommand == nil {
		log.Println("No boot command given, skipping")
		return multistep.ActionContinue
	}

	// Wait the for the vm to boot.
	if int64(s.Config.BootWait) > 0 {
		ui.Say(fmt.Sprintf("Waiting %s for boot...", s.Config.BootWait))
		select {
		case <-time.After(s.Config.BootWait):
			break
		case <-ctx.Done():
			return multistep.ActionHalt
		}
	}

	var pauseFn multistep.DebugPauseFn
	if debug {
		pauseFn = state.Get("pauseFn").(multistep.DebugPauseFn)
	}

	var ip string
	var err error
	port, ok := state.Get("http_port").(int)
	if !ok {
		ui.Error("error retrieving 'http_port' from state")
		return multistep.ActionHalt
	}

	// TODO: Add logic to evaluate whether to use a static IP or DHCP
	c := state.Get("client").(*morpheus.Client)
	instanceStaticIP := ""
	instanceStaticSubnetMask := ""
	instanceStaticGateway := ""
	instanceStaticDNS := ""
	log.Println("NETWORK POOOL ADDRESS: ", instance.Interfaces[0].Network.Pool.ID)
	if instance.Interfaces[0].Network.Pool.ID != 0 {
		ipPoolResponse, err := c.ListNetworkPoolIPAddresses(instance.Interfaces[0].Network.Pool.ID, &morpheus.Request{
			QueryParams: map[string]string{
				"phrase": instance.Name,
			},
		})
		if err != nil {
			log.Println(err)
		}

		// update pool code
		result := ipPoolResponse.Result.(*morpheus.ListNetworkPoolIPAddressesResult)
		ipInfo := (*result.NetworkPoolIps)[0]
		instanceStaticIP = ipInfo.IpAddress
		instanceStaticSubnetMask = ipInfo.SubnetMask
		instanceStaticGateway = ipInfo.GatewayAddress
		instanceStaticDNS = ipInfo.DnsServer
	}
	// If the port is set, we will use the HTTP server to serve the boot command.
	if port > 0 {
		keys := []string{"http_bind_address", "http_interface", "http_ip"}
		for _, key := range keys {
			value, ok := state.Get(key).(string)
			if !ok || value == "" {
				continue
			}

			switch key {
			case "http_bind_address":
				ip = value
				log.Printf("Using IP address %s from %s.", ip, key)
			case "http_interface":
				ip, err = hostIP(value)
				if err != nil {
					err := fmt.Errorf("error using interface %s: %s", value, err)
					state.Put("error", err)
					ui.Errorf("%s", err)
					return multistep.ActionHalt
				}
				log.Printf("Using IP address %s from %s %s.", ip, key, value)
			case "http_ip":
				if err := ValidateHTTPAddress(value); err != nil {
					err := fmt.Errorf("error using IP address %s: %s", value, err)
					state.Put("error", err)
					ui.Errorf("%s", err)
					return multistep.ActionHalt
				}
				ip = value
				log.Printf("Using IP address %s from %s.", ip, key)
			}
		}

		// Check if IP address was determined.
		if ip == "" {
			err := fmt.Errorf("error determining IP address")
			state.Put("error", err)
			ui.Errorf("%s", err)
			return multistep.ActionHalt
		}

		s.Ctx.Data = &bootCommandTemplateData{
			ip,
			port,
			instanceStaticIP,
			instanceStaticSubnetMask,
			instanceStaticGateway,
			instanceStaticDNS,
			s.VMName,
		}

		ui.Sayf("Serving HTTP requests at http://%v:%v/.", ip, port)
	}

	ui.Say("Sending console keys...")

	for _, command := range s.Config.BootCommand {

		command, err := interpolate.Render(command, &s.Ctx)
		if err != nil {
			err := fmt.Errorf("error preparing boot command: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		// Check for interrupts
		if _, ok := state.GetOk(multistep.StateCancelled); ok {
			return multistep.ActionHalt
		}

		// TODO: Add logic to mask sensitive data or not display
		ui.Sayf("Sending console keys: %s", command)
		runCommand(*c, instance, command)
		if pauseFn != nil {
			pauseFn(multistep.DebugLocationAfterRun, fmt.Sprintf("boot_command: %s", command), state)
		}
	}

	// Determines that should continue to the next step
	return multistep.ActionContinue
}

// Cleanup can be used to clean up any artifact created by the step.
// A step's clean up always run at the end of a build, regardless of whether provisioning succeeds or fails.
func (s *StepTypeBootCommand) Cleanup(_ multistep.StateBag) {
	// Nothing to clean
}

func runCommand(c morpheus.Client, instance *morpheus.Instance, command string) {
	payload := make(map[string]interface{})
	payload["sendKeys"] = true
	payload["script"] = command
	resp, err := c.CreateExecutionRequest(&morpheus.Request{
		Body: payload,
		QueryParams: map[string]string{
			"instanceId": strconv.Itoa(int(instance.ID)),
		},
	})
	if err != nil {
		log.Printf("API FAILURE: %s - %s", resp, err)
	}
	log.Printf("API RESPONSE: %s", resp)
	resultGet := resp.Result.(*morpheus.ExecutionRequestResult)
	// TODO: Add boot command interval wait config option

	time.Sleep(5 * time.Second)

	c.GetExecutionRequest(resultGet.ExecutionRequest.UniqueID, &morpheus.Request{})
	log.Println(resp.JsonData)
}

func hostIP(ifname string) (string, error) {
	var addrs []net.Addr
	var err error

	if ifname != "" {
		iface, err := net.InterfaceByName(ifname)
		if err != nil {
			return "", err
		}
		addrs, err = iface.Addrs()
		if err != nil {
			return "", err
		}
	} else {
		addrs, err = net.InterfaceAddrs()
		if err != nil {
			return "", err
		}
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", errors.New("no host IP found")
}
