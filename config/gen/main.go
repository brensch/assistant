package main

import (
	"log/slog"
	"os"

	"github.com/brensch/assistant/config"
	"gopkg.in/yaml.v3"
)

func main() {
	slog.Info("generating empty config")
	var emptyConf config.AppConfig

	confYAML, err := yaml.Marshal(emptyConf)
	if err != nil {
		slog.Error("failed to marshal empty yaml", "err", err)
		return
	}

	err = os.WriteFile("./demo.conf", confYAML, 0644)
	if err != nil {
		slog.Error("failed to write blank conf to file", "err", err)
		return
	}
}
