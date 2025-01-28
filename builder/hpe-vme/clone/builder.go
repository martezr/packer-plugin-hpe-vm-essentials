package clone

import (
	"context"
	"errors"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/martezr/packer-plugin-hpe-vm-essentials/builder/hpe-vme/common"
)

const BuilderId = "hpe-vme-clone.builder"

type Builder struct {
	config Config
	runner multistep.Runner
}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec { return b.config.FlatMapstructure().HCL2Spec() }

func (b *Builder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	// Setup the state bag and initial state for the steps
	state := new(multistep.BasicStateBag)
	state.Put("debug", b.config.PackerDebug)
	state.Put("hook", hook)
	state.Put("ui", ui)

	steps := []multistep.Step{}

	steps = append(steps,
		&common.StepConnect{
			Config: &b.config.ConnectConfiguration,
		},
		&StepProvisionVM{builder: b},
	)

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

	steps = append(steps,
		&common.StepStopInstance{},
		&common.StepConvertInstance{
			ConvertToTemplate: b.config.ConvertToTemplate,
			InstanceName:      b.config.VirtualMachineName,
			TemplateName:      b.config.TemplateName,
		},
		&common.StepRemoveInstance{},
	)

	// Run the steps
	b.runner = commonsteps.NewRunnerWithPauseFn(steps, b.config.PackerConfig, ui, state)
	b.runner.Run(ctx, state)

	// If there was an error, return that
	if err, ok := state.GetOk("error"); ok {
		return nil, err.(error)
	}

	// If we were interrupted or cancelled, then just exit.
	if _, ok := state.GetOk(multistep.StateCancelled); ok {
		return nil, errors.New("build was cancelled")
	}

	artifact := &common.Artifact{
		InstanceId: state.Get("instance_id").(int64),
		// Add the builder generated data to the artifact StateData so that post-processors
		// can access them.
		StateData: map[string]interface{}{
			"generated_data": state.Get("generated_data"),
		},
	}
	return artifact, nil
}
