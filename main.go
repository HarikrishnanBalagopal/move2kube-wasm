package main

import (
	"github.com/konveyor/move2kube-wasm/cmd"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.Infof("start")
	rootCmd := cmd.GetPlanCommand()
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatalf("Error: %q", err)
	}
	logrus.Infof("end")
}
