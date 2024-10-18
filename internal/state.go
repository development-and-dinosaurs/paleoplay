package internal

import (
	"io"
	"log"
	"os"
	"slices"
	"strings"
)

func ReadState() (state []string) {
	stateFile, err := os.OpenFile(".state", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0777)
	defer stateFile.Close()
	if err != nil {
		log.Fatalf("could not read state file: %v", err)
	}
	stateContents, err := io.ReadAll(stateFile)
	if err != nil {
		log.Fatalf("could not read state file: %v", err)
	}
	state = strings.Split(string(stateContents), "\n")
	return state
}

func WriteState(state []string) {
	slices.Sort(state)
	stateFile, err := os.OpenFile(".state", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0777)
	defer stateFile.Close()
	for _, s := range state {
		_, err = stateFile.WriteString(s + "\n")
		if err != nil {
			log.Fatalf("could not write state: %v", err)
		}
	}
}
