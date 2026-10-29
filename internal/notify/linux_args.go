package notify

// notifySendArgs is untagged so it is tested on every OS.
func notifySendArgs(r Request) []string {
	urgency := "normal"
	if r.Urgent {
		urgency = "critical"
	}
	return []string{"--app-name=Claude", "--urgency=" + urgency, "--expire-time=10000", r.Title, r.Body}
}
