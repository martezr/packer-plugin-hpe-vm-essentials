//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,NetworkInterface,StorageVolume

package iso

import (
	"time"

	packerCommon "github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/martezr/packer-plugin-hpe-vm-essentials/builder/hpe-vme/common"
)

type Config struct {
	packerCommon.PackerConfig   `mapstructure:",squash"`
	commonsteps.HTTPConfig      `mapstructure:",squash"`
	BootConfig                  `mapstructure:",squash"`
	Comm                        communicator.Config `mapstructure:",squash"`
	common.ConnectConfiguration `mapstructure:",squash"`
	// Amount of time to wait for VM's IP, similar to 'ssh_timeout'.
	// Defaults to 30m (30 minutes). See the Golang
	// [ParseDuration](https://golang.org/pkg/time/#ParseDuration) documentation
	// for full details.
	IPWaitTimeout         time.Duration `mapstructure:"ip_wait_timeout"`
	HTTPTemplateDirectory string        `mapstructure:"http_template_directory"`
	// Whether to convert the instance to a virtual image
	ConvertToTemplate bool `mapstructure:"convert_to_template"`
	// The name of the HPE VM Essentials cluster to provision the instance on.
	ClusterName string `mapstructure:"cluster_name" required:"true"`
	// The name of the instance to provision.
	VirtualMachineName string `mapstructure:"vm_name" required:"true"`
	// The ID of the ISO virtual image to use as the instance instance source image.
	VirtualImageID int64 `mapstructure:"virtual_image_id" required:"true"`
	// The name of the virtual image to create.
	TemplateName string `mapstructure:"template_name"`
	// The labels associated with the virtual image.
	TemplateLabels []string `mapstructure:"template_labels"`
	// The minimum amount of memory required to provision the virtual image.
	TemplateMinimumMemory int64 `mapstructure:"template_minimum_memory"`
	// Whether cloud init is enabled on the virtual image.
	TemplateCloudInitEnabled bool `mapstructure:"template_cloud_init_enabled"`
	// The ID of the storage bucket to store the virtual image.
	TemplateStorageBucketId int64 `mapstructure:"template_storage_bucket_id"`
	// The ID of the service plan that will be associated with the instance.
	ServicePlanID int64 `mapstructure:"plan_id" required:"true"`
	// The name of the cloud that contains the HPE VM Essentials cluster.
	Cloud string `mapstructure:"cloud" required:"true"`
	// The VM Essentials group to associate the instance with.
	Group string `mapstructure:"group" required:"true"`
	// The description of the instance to provision.
	Description string `mapstructure:"description"`
	// The environment to associate with the instance.
	Environment string `mapstructure:"environment"`
	// The metadata labels to associate with the instance.
	Labels            []string                 `mapstructure:"labels"`
	Tags              []map[string]interface{} `mapstructure:"tags"`
	NetworkInterfaces []NetworkInterface       `mapstructure:"network_interface" required:"true"`
	StorageVolumes    []StorageVolume          `mapstructure:"storage_volume" required:"true"`
	// The ID of the HPE VM Essentials host to deploy the instance to.
	HostID int64 `mapstructure:"host"`
	// Whether to attach the VirtIO drivers ISO to the instance.
	AttachVirtioDrivers bool                `mapstructure:"attach_virtio_drivers"`
	Ctx                 interpolate.Context `mapstructure-to-hcl2:",skip"`
}

type NetworkInterface struct {
	// The name of the network to connect the interface to.
	Network string `mapstructure:"network" required:"true"`
	// The ID of the network interface type used by the network interface.
	NetworkInterfaceTypeId int64 `mapstructure:"network_interface_type_id" required:"true"`
}

type StorageVolume struct {
	// The name of the storage volume.
	Name string `mapstructure:"name"`
	// Whether the storage volume is the root volume.
	RootVolume bool `mapstructure:"root_volume"`
	// The size in GB of the storage volume.
	Size int64 `mapstructure:"size"`
	// The ID of the storage type for the storage volume.
	StorageTypeID int64 `mapstructure:"storage_type_id"`
	// The ID of the datastore for the storage volume.
	DatastoreID string `mapstructure:"datastore_id"`
}

func (c *Config) Prepare(raws ...interface{}) (generatedVars []string, warnings []string, err error) {
	err = config.Decode(c, &config.DecodeOpts{
		PluginType:         "mvm",
		Interpolate:        true,
		InterpolateContext: &c.Ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{
				"boot_command",
			},
		},
	}, raws...)
	if err != nil {
		return nil, nil, err
	}

	var errs *packersdk.MultiError

	errs = packersdk.MultiErrorAppend(errs, c.Comm.Prepare(&c.Ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.BootConfig.Prepare(&c.Ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.HTTPConfig.Prepare(&c.Ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.ConnectConfiguration.Prepare()...)

	if errs != nil && len(errs.Errors) > 0 {
		return nil, warnings, errs
	}
	return nil, warnings, nil
}
