package iso

import (
	"context"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/martezr/packer-plugin-hpe-vm-essentials/builder/hpe-vme/common"
)

const BuilderId = "hpe-vme-iso.builder"

type Builder struct {
	config Config
	runner multistep.Runner
}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec { return b.config.FlatMapstructure().HCL2Spec() }

func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	return b.config.Prepare(raws...)
}

func (b *Builder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	// Setup the state bag and initial state for the steps
	state := new(multistep.BasicStateBag)
	state.Put("debug", b.config.PackerDebug)
	state.Put("hook", hook)
	state.Put("ui", ui)

	steps := []multistep.Step{}

	// Set the address for the HTTP server based on the configuration
	// provided by the user.
	if addrs := b.config.HTTPConfig.HTTPAddress; addrs != "" && addrs != DefaultHttpBindAddress {
		// Validate and use the specified HTTPAddress.
		err := ValidateHTTPAddress(addrs)
		if err != nil {
			ui.Errorf("error validating IP address for HTTP server: %s", err)
			return nil, err
		}
		state.Put("http_bind_address", addrs)
	} else if intf := b.config.HTTPConfig.HTTPInterface; intf != "" {
		// Use the specified HTTPInterface.
		state.Put("http_interface", intf)
	}

	steps = append(steps,
		&common.StepConnect{
			Config: &b.config.ConnectConfiguration,
		},
		&StepProvisionVM{builder: b},
		// TODO: Add a check to evaluate if the variable is defined
		&StepGenerateHTTPTemplates{
			TemplateDirectory: b.config.HTTPTemplateDirectory,
			HTTPDirectory:     b.config.HTTPDir,
			Ctx:               b.config.Ctx,
		},
		commonsteps.HTTPServerFromHTTPConfig(&b.config.HTTPConfig),
		&StepTypeBootCommand{
			Config: &b.config.BootConfig,
			Ctx:    b.config.Ctx,
			VMName: b.config.VirtualMachineName,
		},
		&common.StepWaitForIp{
			IPWaitTimeout: b.config.IPWaitTimeout,
		},
	)

	// TODO: Add support for WinRM communicator

	if b.config.Comm.Type == "ssh" {
		steps = append(steps,
			&communicator.StepConnect{
				Config:    &b.config.Comm,
				Host:      communicator.CommHost(b.config.Comm.Host(), "instance_ip"),
				SSHConfig: b.config.Comm.SSHConfigFunc(),
			},
			&commonsteps.StepProvision{},
		)
	}

	if b.config.Comm.Type == "none" {
		steps = append(steps,
			&commonsteps.StepProvision{},
		)
	}

	if b.config.ConvertToTemplate {
		steps = append(steps,
			&common.StepStopInstance{},
			&common.StepEjectDisks{},
			&common.StepConvertInstance{
				ConvertToTemplate: b.config.ConvertToTemplate,
				InstanceName:      b.config.VirtualMachineName,
				TemplateName:      b.config.TemplateName,
			},
			&common.StepRemoveInstance{},
		)
	}

	// Set the value of the generated data that will become available to provisioners.
	// To share the data with post-processors, use the StateData in the artifact.
	state.Put("generated_data", map[string]interface{}{
		"GeneratedMockData": "mock-build-data",
	})

	// Run!
	b.runner = commonsteps.NewRunnerWithPauseFn(steps, b.config.PackerConfig, ui, state)
	b.runner.Run(ctx, state)

	// If there was an error, return that
	if err, ok := state.GetOk("error"); ok {
		return nil, err.(error)
	}

	artifact := &common.Artifact{
		// Add the builder generated data to the artifact StateData so that post-processors
		// can access them.
		StateData: map[string]interface{}{
			"generated_data": state.Get("generated_data"),
		},
	}
	return artifact, nil
}
