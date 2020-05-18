package utils

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
)

func SplitCommand(cmd string) ([]string, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil, fmt.Errorf("cmd is empty")
	}
	r := csv.NewReader(bytes.NewBufferString(cmd))
	r.Comma = ' '
	r.TrimLeadingSpace = true
	line, err := r.Read()
	if err != nil {
		return nil, err
	}
	return line, nil
}
