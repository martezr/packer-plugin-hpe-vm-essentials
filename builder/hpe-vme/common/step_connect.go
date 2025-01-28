package common

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/gomorpheus/morpheus-go-sdk"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// This is a definition of a builder step and should implement multistep.Step
type ConnectConfiguration struct {
	Url         string `mapstructure:"url"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	AccessToken string `mapstructure:"access_token"`
}

type StepConnect struct {
	Config *ConnectConfiguration
}

// Run should execute the purpose of this step
func (s *StepConnect) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := morpheus.NewClient(s.Config.Url)
	if s.Config.AccessToken != "" {
		client.SetAccessToken(s.Config.AccessToken, "", 86400, "write")
	} else {
		client.SetUsernameAndPassword(s.Config.Username, s.Config.Password)
	}
	resp, err := client.Login()
	if err != nil {
		fmt.Println("LOGIN ERROR: ", err)
	}
	fmt.Println("LOGIN RESPONSE:", resp)

	state.Put("client", client)
	// Determines that should continue to the next step
	return multistep.ActionContinue
}

// Cleanup can be used to clean up any artifact created by the step.
// A step's clean up always run at the end of a build, regardless of whether provisioning succeeds or fails.
func (s *StepConnect) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	c, ok := state.GetOk("client")
	if !ok {
		log.Printf("[INFO] No client in state; nothing to cleanup.")
		return
	}

	client, ok := c.(*morpheus.Client)
	if !ok {
		log.Printf("[ERROR] The object stored in the state under 'client' key is of type '%s', not 'morpheus.client'. This could indicate a problem with the state initialization or management.", reflect.TypeOf(c))
		return
	}
	ui.Message("Closing sessions ....")
	client.Logout()
}
