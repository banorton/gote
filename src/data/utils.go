package data

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

func PrettyPrintJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(v)
		return
	}
	fmt.Println(string(data))
}

func OpenFileInEditor(filePath, editor string) error {
	if editor == "" {
		return fmt.Errorf("no editor specified in config")
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error opening editor: %w", err)
	}
	return nil
}

