package tea

func init() {
	// zscalerctl patch: the upstream Bubble Tea v1.x init called
	// lipgloss.HasDarkBackground(), which emits OSC/DSR terminal probes before
	// main() runs. That caused hangs in zscalerctl-tui failure paths such as
	// `zscalerctl-tui --live --profile definitely-not-real`, where the binary
	// should fail promptly but instead blocked on a terminal query.
	//
	// The patched init does nothing. The zscalerctl-tui binary does not rely on
	// Bubble Tea's startup background-color detection; styling is decided by the
	// existing output.ShouldColor gate after main() runs.
	//
	// This workaround will be removed when Bubble Tea v2 is adopted upstream.
}
