package common

import (
	"context"
	"log"
	"time"

	"github.com/gomorpheus/morpheus-go-sdk"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// This is a definition of a builder step and should implement multistep.Step
type StepRemoveInstance struct{}

// Run should execute the purpose of this step
func (s *StepRemoveInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	var (
		instance = state.Get("instance").(*morpheus.Instance)
		ui       = state.Get("ui").(packersdk.Ui)
	)

	ui.Say("Removing instance")
	c := state.Get("client").(*morpheus.Client)

	data, err := c.DeleteInstance(instance.ID, &morpheus.Request{})
	if err != nil {
		log.Println(err)
	}

	log.Println(data.Status)
	// TODO: Add polling support to check instance state
	time.Sleep(30 * time.Second)

	// Determines that should continue to the next step
	return multistep.ActionContinue
}

// Cleanup can be used to clean up any artifact created by the step.
// A step's clean up always run at the end of a build, regardless of whether provisioning succeeds or fails.
func (s *StepRemoveInstance) Cleanup(_ multistep.StateBag) {
	// Nothing to clean
}
