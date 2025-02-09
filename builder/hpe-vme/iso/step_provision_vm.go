package iso

import (
	"context"
	"encoding/json"
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
	DatastoreId string `json:"datastoreId"`
}

// Run should execute the purpose of this step
func (s *StepProvisionVM) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {

	var err error

	ui := state.Get("ui").(packersdk.Ui)

	// Config
	config := make(map[string]interface{})

	c := state.Get("client").(*morpheus.Client)

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

	// Image ID
	config["imageId"] = s.builder.config.VirtualImageID

	// Host Id
	config["kvmHostId"] = s.builder.config.HostID

	// Attach VirtIO Drivers
	config["attachVirtIODrivers"] = s.builder.config.AttachVirtioDrivers

	// Skip Agent Install
	config["noAgent"] = true

	// Skip Backup Creation
	config["createBackup"] = false

	groupResp, err := c.FindGroupByName(s.builder.config.Group)
	if err != nil {
		ui.Error(err.Error())
		log.Printf("API FAILURE: %s - %s", groupResp, err)
	}
	group := groupResp.Result.(*morpheus.GetGroupResult)

	var vmTags []map[string]interface{}
	for key, value := range s.builder.config.Tags {
		tag := make(map[string]interface{})
		tag["name"] = key
		tag["value"] = value
		vmTags = append(vmTags, tag)
	}

	instancePayload := map[string]interface{}{
		"name":            s.builder.config.VirtualMachineName,
		"type":            "mvm",
		"description":     s.builder.config.Description,
		"instanceContext": s.builder.config.Environment,
		"labels":          s.builder.config.Labels,
		"tags":            vmTags,
		"instanceType": map[string]interface{}{
			"code": "mvm",
		},
		"site": map[string]interface{}{
			"id": group.Group.ID,
		},
		"plan": map[string]interface{}{
			"id": s.builder.config.ServicePlanID,
		},
		// How to find the instance layout id
		"layout": map[string]interface{}{
			//	"name":              "Single HPE VM",
			//	"provisionTypeCode": "kvm",
			"id": 2,
		},
	}

	cloudResp, err := c.FindCloudByName(s.builder.config.Cloud)
	if err != nil {
		ui.Error(err.Error())
		log.Printf("API FAILURE: %s - %s", cloudResp, err)
	}
	cloud := cloudResp.Result.(*morpheus.GetCloudResult)

	payload := map[string]interface{}{
		"zoneId":   cloud.Cloud.ID,
		"instance": instancePayload,
		"config":   config,
	}

	// Network Interfaces
	var Nics []PayloadNetworkInterface

	for _, nic := range s.builder.config.NetworkInterfaces {
		resp, err := c.ListNetworks(&morpheus.Request{
			QueryParams: map[string]string{
				"name": nic.Network,
			},
		})
		if err != nil {
			ui.Error(err.Error())
			log.Printf("API FAILURE: %s - %s", cloudResp, err)
		}
		networks := resp.Result.(*morpheus.ListNetworksResult)
		networkId := 0
		for _, network := range *networks.Networks {
			if network.ZonePool.Name == s.builder.config.ClusterName {
				networkId = int(network.ID)
			}
		}

		// Error out if the defined network is unable to be found
		if networkId == 0 {
			ui.Errorf("Unable to find network named %s", nic.Network)
			ui.Error(err.Error())
		}
		var NetworkData PayloadNetworkInterface
		NetworkData.NetworkInterfaceTypeID = nic.NetworkInterfaceTypeId
		NetworkData.Network.ID = fmt.Sprintf("network-%d", networkId)
		Nics = append(Nics, NetworkData)
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
	//out, _ := json.Marshal(payload)
	//fmt.Println(out)

	req := &morpheus.Request{Body: payload}
	createInstanceresp, err := c.CreateInstance(req)
	if err != nil {
		ui.Error(err.Error())
		log.Printf("API FAILURE: %s - %s", createInstanceresp, err)
	}
	log.Printf("API RESPONSE: %s", createInstanceresp)
	result := createInstanceresp.Result.(*morpheus.CreateInstanceResult)
	instance := result.Instance

	// Status List: provisioning, pending, removing
	// Poll Instance for status
	currentStatus := "provisioning"
	completedStatuses := []string{"running", "failed", "warning", "denied", "cancelled", "suspended"}
	ui.Sayf("Waiting for instance (%d) to become ready", instance.ID)

	for !stringInSlice(completedStatuses, currentStatus) {
		// sleep 5 seconds between polls
		time.Sleep(5 * time.Second)
		resp, err := c.GetInstance(instance.ID, &morpheus.Request{})
		if err != nil {
			log.Println("API ERROR: ", err)
		}
		result := resp.Result.(*morpheus.GetInstanceResult)
		currentStatus = result.Instance.Status
		ui.Sayf("Waiting for instance to provision - %s", currentStatus)
	}

	ui.Sayf("Instance (%d) has become ready", instance.ID)
	respGet, err := c.GetInstance(instance.ID, req)
	if err != nil {
		ui.Error(err.Error())
		log.Printf("API FAILURE: %s - %s", respGet, err)
	}
	log.Printf("API RESPONSE: %s", respGet)
	resultGet := respGet.Result.(*morpheus.GetInstanceResult)
	instanceGet := resultGet.Instance

	state.Put("instance", instanceGet)
	state.Put("instance_id", instance.ID)

	ui.Sayf("Instance Status: (%s)", instanceGet.Status)

	if instanceGet.Status != "running" {
		ui.Error("Instance was unable to successfully provision")
		return multistep.ActionHalt
	}

	// Determines that should continue to the next step
	return multistep.ActionContinue
}

// Cleanup can be used to clean up any artifact created by the step.
// A step's clean up always run at the end of a build, regardless of whether provisioning succeeds or fails.
func (s *StepProvisionVM) Cleanup(state multistep.StateBag) {
	/*
	   instance := state.Get("instance").(*morpheus.Instance)
	   ui := state.Get("ui").(packersdk.Ui)

	   ui.Say("Removing instance due to a failed build")
	   c := state.Get("client").(*morpheus.Client)

	   data, err := c.DeleteInstance(instance.ID, &morpheus.Request{})

	   	if err != nil {
	   		ui.Error(err.Error())
	   		log.Println(err)
	   	}

	   log.Println(data.Status)
	   // TODO: Add polling support to check instance state
	   time.Sleep(30 * time.Second)
	*/
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
