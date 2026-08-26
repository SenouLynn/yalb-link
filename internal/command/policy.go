package command

import (
	"fmt"

	"github.com/bluenviron/gomavlib/v3/pkg/message"

	"yalb.gcs/internal/codec"
)

func encodeArm(sysID uint8, arm bool) (message.Message, error) {
	if sysID == 0 {
		return nil, fmt.Errorf("command: system ID must be between 1 and 255")
	}
	var param1 float32
	if arm {
		param1 = 1
	}
	// param2 is deliberately zero: force-arm magic is not part of policy.
	return codec.EncodeCommandLong(codec.Target{
		SystemID: sysID, ComponentID: codec.AutopilotComponentID,
	}, codec.CmdComponentArmDisarm, 0, [7]float32{param1, 0, 0, 0, 0, 0, 0}), nil
}
