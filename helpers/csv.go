package helpers

import (
	"bytes"
	"os"
)

func CleanCSVFile(filePath string) error {
	input, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	outFile := filePath + "_out"
	output := bytes.Replace(input, []byte(`"`), []byte(""), -1)

	if err = os.WriteFile(outFile, output, 0666); err != nil {
		return err
	}

	err = os.Rename(outFile, filePath)
	if err != nil {
		return err
	}

	return nil
}
