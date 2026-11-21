package coreserver

func effectiveMattingDevice(device string) string {
	if device == "cpu" {
		return "cpu"
	}
	return "gpu"
}
