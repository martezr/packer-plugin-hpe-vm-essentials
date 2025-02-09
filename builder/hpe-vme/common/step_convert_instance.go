package common

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gomorpheus/morpheus-go-sdk"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// This is a definition of a builder step and should implement multistep.Step
type StepConvertInstance struct {
	ConvertToTemplate        bool
	InstanceName             string
	TemplateName             string
	TemplateMinimumMemory    int64
	TemplateCloudInitEnabled bool
	TemplateLabels           []string
	TemplateStorageBucketId  int64
}

// Run should execute the purpose of this step
func (s *StepConvertInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	var (
		instance = state.Get("instance").(*morpheus.Instance)
		ui       = state.Get("ui").(packersdk.Ui)
	)

	if s.ConvertToTemplate {
		ui.Say("Converting instance to virtual image")
		c := state.Get("client").(*morpheus.Client)

		clonePayload := make(map[string]interface{})
		clonePayload["templateName"] = s.TemplateName
		clonePayload["storageProviderId"] = s.TemplateStorageBucketId
		data, err := c.Execute(&morpheus.Request{
			Method: "PUT",
			Body:   clonePayload,
			Path:   fmt.Sprintf("/api/instances/%d/import-snapshot", instance.ID),
		})
		if err != nil {
			log.Println(err)
		}

		log.Println(data.JsonData)
		// TODO: Add polling for image existance
		time.Sleep(150 * time.Second)

		imageResponse, imageErr := c.ListVirtualImages(&morpheus.Request{
			QueryParams: map[string]string{
				"name": s.TemplateName,
			},
		})

		if imageErr != nil {
			log.Println(imageErr)
		}
		result := imageResponse.Result.(*morpheus.ListVirtualImagesResult)
		if len(*result.VirtualImages) == 0 {
			failUrl := fmt.Sprintf("unable to find virtual image %s", s.TemplateName)
			var failError = errors.New(failUrl)
			state.Put("error", failError)
			ui.Error(failUrl)
			return multistep.ActionHalt
		}

		firstRecord := (*result.VirtualImages)[0]
		log.Println("IMAGE STATUS: ", firstRecord)
		virtualImageId := firstRecord.ID

		// Status List: provisioning, pending, cancelled, removing
		// Poll Virtual Images for Status
		currentStatus := "Saving"
		completedStatuses := []string{"Active"}

		for !stringInSlice(completedStatuses, currentStatus) {
			resp, err := c.GetVirtualImage(virtualImageId, &morpheus.Request{})
			if err != nil {
				log.Println("API ERROR: ", err)
			}
			result := resp.Result.(*morpheus.GetVirtualImageResult)
			currentStatus = result.VirtualImage.Status
			log.Println("Current status:", currentStatus)
			time.Sleep(15 * time.Second)
		}

		resp, err := c.UpdateVirtualImage(virtualImageId, &morpheus.Request{
			Body: map[string]interface{}{
				"virtualImage": map[string]interface{}{
					"isCloudInit":      s.TemplateCloudInitEnabled,
					"vmToolsInstalled": false,
					"labels":           s.TemplateLabels,
					"installAgent":     false,
					"sshUsername":      "",
					"sshPassword":      "",
				},
			},
		})
		if err != nil {
			log.Println("API ERROR: ", err)
		}
		log.Printf("API RESPONSE: %s", resp)

	}
	// Determines that should continue to the next step
	return multistep.ActionContinue
}

// Cleanup can be used to clean up any artifact created by the step.
// A step's clean up always run at the end of a build, regardless of whether provisioning succeeds or fails.
func (s *StepConvertInstance) Cleanup(_ multistep.StateBag) {
	// Nothing to clean
}

func stringInSlice(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
