package notify

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
)

// readAnswer returns the answer the helper wrote and deletes the file. The
// helper writes {"action":"error"} on failure (`open -W` hides its exit code),
// which yields nil. No file after a clean exit means nobody answered.
func readAnswer(file string, exitOK bool) *Answer {
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		if exitOK {
			return &Answer{Action: "timeout"}
		}
		return nil
	}
	if err != nil {
		return nil
	}
	os.Remove(file)
	var a struct{ Action, Text string }
	if json.Unmarshal(data, &a) != nil || a.Action == "error" {
		return nil
	}
	return &Answer{Action: a.Action, Text: a.Text}
}
