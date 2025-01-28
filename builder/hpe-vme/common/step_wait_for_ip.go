package common

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gomorpheus/morpheus-go-sdk"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// This is a definition of a builder step and should implement multistep.Step
type StepWaitForIp struct {
	IPWaitTimeout time.Duration
}

// Run should execute the purpose of this step
func (s *StepWaitForIp) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	c := state.Get("client").(*morpheus.Client)
	instance := state.Get("instance").(*morpheus.Instance)
	ui := state.Get("ui").(packersdk.Ui)
	ui.Say("Waiting for instance IP address")

	var ip string
	var err error

	sub, cancel := context.WithCancel(ctx)
	waitDone := make(chan bool, 1)
	defer func() {
		cancel()
	}()

	go func() {
		ip, err = fetchIP(sub, *c, *instance)
		waitDone <- true
	}()
	timeout := time.After(s.IPWaitTimeout)
	for {
		select {
		case <-timeout:
			cancel()
			<-waitDone
			err := fmt.Errorf("timeout after %s waiting for IP address", s.IPWaitTimeout)
			state.Put("error", err)
			ui.Errorf("%s", err)
			return multistep.ActionHalt
		case <-ctx.Done():
			cancel()
			log.Println("[WARN] Interrupt detected, quitting waiting for IP.")
			return multistep.ActionHalt
		case <-waitDone:
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
			state.Put("instance_ip", ip)
			ui.Sayf("Instance IP address: %v", ip)
			return multistep.ActionContinue
		case <-time.After(1 * time.Second):
			if _, ok := state.GetOk(multistep.StateCancelled); ok {
				return multistep.ActionHalt
			}
		}
	}
}

// Cleanup can be used to clean up any artifact created by the step.
// A step's clean up always run at the end of a build, regardless of whether provisioning succeeds or fails.
func (s *StepWaitForIp) Cleanup(_ multistep.StateBag) {
	// Nothing to clean
}

func fetchIP(ctx context.Context, c morpheus.Client, instance morpheus.Instance) (string, error) {
	for {
		time.Sleep(5 * time.Second)
		resp, err := c.GetInstance(instance.ID, &morpheus.Request{})
		if err != nil {
			log.Println("API ERROR: ", err)
		}
		result := resp.Result.(*morpheus.GetInstanceResult)
		instanceIP := result.Instance.ConnectionInfo[0].Ip
		if instanceIP != "0.0.0.0" {
			return instanceIP, nil
		}

		// Check for ctx cancellation to avoid printing any IP logs at the timeout
		select {
		case <-ctx.Done():
			return instanceIP, fmt.Errorf("cancelled waiting for IP address")
		default:
		}
	}
}
