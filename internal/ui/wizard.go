package ui

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"kian/internal/audio"
)

// showDialog displays a native macOS dialog via osascript and returns
// the button the user clicked. Returns empty string on error.
func showDialog(title, message string, buttons []string) string {
	btnList := `{"` + strings.Join(buttons, `", "`) + `"}`
	script := fmt.Sprintf(
		`tell application "System Events" to set frontmost of process "Kian" to true
display dialog %q buttons %s default button %q with title %q with icon note`,
		message, btnList, buttons[len(buttons)-1], title,
	)

	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return ""
	}

	result := strings.TrimSpace(string(out))
	if strings.HasPrefix(result, "button returned:") {
		return strings.TrimPrefix(result, "button returned:")
	}
	return ""
}

// RunSetupWizard checks for missing audio devices and creates them if needed.
// It shows native macOS dialogs to guide the user through the process.
func RunSetupWizard() {
	status := audio.CheckDevices()

	if !status.BlackHoleFound {
		for {
			btn := showDialog("Kian — BlackHole Required",
				"BlackHole 2ch virtual audio driver is required.\n\n"+
					"Open Terminal and run:\n"+
					"brew install blackhole-2ch\n\n"+
					"Then click Retry.",
				[]string{"Cancel", "Open Terminal", "Retry"})

			if btn == "Open Terminal" {
				exec.Command("open", "-a", "Terminal").Start()
				continue
			}
			if btn == "Retry" {
				status = audio.CheckDevices()
				if status.BlackHoleFound {
					break
				}
				continue
			}
			return
		}
	}

	var errors []string

	if !status.MultiOutputFound {
		if err := audio.CreateKianMultiOutputDevice(); err != nil {
			log.Printf("create multi-output device: %v", err)
			errors = append(errors, fmt.Sprintf("Multi-Output Device: %v", err))
		}
	}

	if !status.AggregateFound {
		if err := audio.CreateKianAggregateDevice(); err != nil {
			log.Printf("create aggregate device: %v", err)
			errors = append(errors, fmt.Sprintf("Aggregate Device: %v", err))
		}
	}

	if len(errors) > 0 {
		showDialog("Kian — Setup Error",
			"Some devices could not be created:\n\n"+
				strings.Join(errors, "\n")+
				"\n\nYou can set them up manually in Audio MIDI Setup.",
			[]string{"OK"})
		return
	}

	showDialog("Kian — Setup Complete",
		"Audio devices created successfully!\n\n"+
			"One last step: go to System Settings → Sound and set "+
			"\"Kian Multi-Output\" as your sound output device. "+
			"This routes system audio through BlackHole for capture.",
		[]string{"OK"})
}
