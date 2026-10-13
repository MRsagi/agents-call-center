package focus

// NewOS returns a Focuser that drives the host OS (Accessibility / foreground window).
func NewOS() Focuser {
	return newOSImpl()
}
