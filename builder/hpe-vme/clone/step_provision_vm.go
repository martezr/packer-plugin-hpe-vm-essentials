package clone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gomorpheus/morpheus-go-sdk"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// This is a definition of a builder step and should implement multistep.Step
type StepProvisionVM struct {
	builder *Builder
}

type PayloadNetworkInterface struct {
	Network struct {
		ID string `json:"id"`
	} `json:"network"`
	NetworkInterfaceTypeID int64 `json:"networkInterfaceTypeID"`
}

type PayloadStorageVolume struct {
	ID          int64  `json:"id"`
	RootVolume  bool   `json:"rootVolume"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	StorageType int64  `json:"storageType"`
	DatastoreId int64  `json:"datastoreId"`
}

// Run should execute the purpose of this step
func (s *StepProvisionVM) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)

	// Config
	config := make(map[string]interface{})

	// TODO: Update the instance to MVM
	c := state.Get("client").(*morpheus.Client)
	instanceTypeResponse, err := c.FindInstanceTypeByName("mvm")
	if err != nil {
		log.Printf("API FAILURE: %s - %s", instanceTypeResponse, err)
	}
	log.Printf("API RESPONSE: %s", instanceTypeResponse)
	instanceTypeResult := instanceTypeResponse.Result.(*morpheus.GetInstanceTypeResult)
	instanceType := instanceTypeResult.InstanceType

	// Resource Pool
	resourcePoolResp, err := c.Execute(&morpheus.Request{
		Method: "GET",
		//		Path:        fmt.Sprintf("/api/options/zonePools?layoutId=%d", instanceType.InstanceTypeLayouts[0].ID),
		Path:        fmt.Sprintf("/api/options/zonePools"),
		QueryParams: map[string]string{},
	})
	if err != nil {
		ui.Error(err.Error())
		log.Println(err)
	}

	var itemResponsePayload ResourcePoolOptions
	json.Unmarshal(resourcePoolResp.Body, &itemResponsePayload)
	var resourcePoolId int
	for _, v := range itemResponsePayload.Data {
		if v.ProviderType == "mvm" && v.Name == s.builder.config.ClusterName {
			resourcePoolId = v.Id
		}
	}

	config["resourcePoolId"] = resourcePoolId
	config["poolProviderType"] = "mvm"
	// Create User
	config["createUser"] = true

	// Image ID
	config["imageId"] = s.builder.config.VirtualImageID

	// Skip Agent Install
	config["noAgent"] = s.builder.config.SkipAgentInstall

	instancePayload := map[string]interface{}{
		"name": s.builder.config.VirtualMachineName,
		"type": instanceType.Code,
		"instanceType": map[string]interface{}{
			"code": instanceType.Code,
		},
		"site": map[string]interface{}{
			"id": s.builder.config.GroupID,
		},
		"plan": map[string]interface{}{
			"id": s.builder.config.ServicePlanID,
		},
		"layout": map[string]interface{}{
			"id": instanceType.InstanceTypeLayouts[0].ID,
		},
	}

	payload := map[string]interface{}{
		"zoneId":   s.builder.config.Cloud,
		"instance": instancePayload,
		"config":   config,
	}

	// Network Interfaces
	var Nics []PayloadNetworkInterface

	for _, nic := range s.builder.config.NetworkInterfaces {
		var NetworkDemo PayloadNetworkInterface
		NetworkDemo.NetworkInterfaceTypeID = nic.NetworkInterfaceTypeId
		NetworkDemo.Network.ID = fmt.Sprintf("network-%d", nic.NetworkId)
		Nics = append(Nics, NetworkDemo)
	}
	payload["networkInterfaces"] = Nics

	// Storage Volumes
	var Volumes []PayloadStorageVolume

	for _, sv := range s.builder.config.StorageVolumes {
		var StorageDemo PayloadStorageVolume
		StorageDemo.ID = -1
		StorageDemo.Name = sv.Name
		StorageDemo.RootVolume = sv.RootVolume
		StorageDemo.Size = sv.Size
		StorageDemo.StorageType = sv.StorageTypeID
		StorageDemo.DatastoreId = sv.DatastoreID
		Volumes = append(Volumes, StorageDemo)
	}

	payload["volumes"] = Volumes
	payload["layoutSize"] = 1

	// TODO: Remove additional logging
	out, _ := json.Marshal(payload)
	fmt.Println(out)

	req := &morpheus.Request{Body: payload}
	resp, err := c.CreateInstance(req)
	if err != nil {
		log.Printf("API FAILURE: %s - %s", resp, err)
	}
	log.Printf("API RESPONSE: %s", resp)
	result := resp.Result.(*morpheus.CreateInstanceResult)
	instance := result.Instance

	// Status List: provisioning, pending, cancelled, removing
	// Poll Instance for Status
	currentStatus := "provisioning"
	completedStatuses := []string{"running", "failed", "warning", "denied", "cancelled", "suspended"}
	ui.Sayf("Waiting for instance %s (id: %d) to become ready", instance.Name, instance.ID)

	for !stringInSlice(completedStatuses, currentStatus) {
		resp, err := c.GetInstance(instance.ID, &morpheus.Request{})
		if err != nil {
			log.Println("API ERROR: ", err)
		}
		result := resp.Result.(*morpheus.GetInstanceResult)
		currentStatus = result.Instance.Status
		ui.Sayf("Waiting for instance to provision - current status: %s", currentStatus)
		// sleep 30 seconds between polls
		time.Sleep(30 * time.Second)
	}

	respGet, err := c.GetInstance(instance.ID, &morpheus.Request{})
	if err != nil {
		log.Printf("API FAILURE: %s - %s", respGet, err)
	}
	log.Printf("API RESPONSE: %s", respGet)
	resultGet := respGet.Result.(*morpheus.GetInstanceResult)
	instanceGet := resultGet.Instance

	if instanceGet.Status == "failed" {
		failUrl := fmt.Sprintf("Instance provisioning failed - %s/provisioning/instances/%d", s.builder.config.Url, instance.ID)
		var ErrMyError = errors.New(failUrl)
		state.Put("error", ErrMyError)
		ui.Error(failUrl)
		return multistep.ActionHalt
	}

	state.Put("instance", instanceGet)
	state.Put("instance_ip", instanceGet.ConnectionInfo[0].Ip)
	state.Put("instance_id", instance.ID)

	ui.Say(fmt.Sprintf("Instance IP Address %s", instanceGet.ConnectionInfo[0].Ip))

	// Determines that should continue to the next step
	return multistep.ActionContinue
}

// Cleanup can be used to clean up any artifact created by the step.
// A step's clean up always run at the end of a build, regardless of whether provisioning succeeds or fails.
func (s *StepProvisionVM) Cleanup(state multistep.StateBag) {
	instanceId := state.Get("instance_id")
	if instanceId == nil {
		return
	}
	c := state.Get("client").(*morpheus.Client)
	data, err := c.DeleteInstance(instanceId.(int64), &morpheus.Request{})
	if err != nil {
		log.Println(err)
	}

	log.Println(data.Status)
	time.Sleep(30 * time.Second)
}

type ResourcePoolOptions struct {
	Success bool `json:"success"`
	Data    []struct {
		Id           int    `json:"id"`
		Name         string `json:"name"`
		IsGroup      bool   `json:"isGroup"`
		Group        string `json:"group"`
		IsDefault    bool   `json:"isDefault"`
		Type         string `json:"type"`
		ProviderType string `json:"providerType"`
		Value        string `json:"value"`
	} `json:"data"`
}

func stringInSlice(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
