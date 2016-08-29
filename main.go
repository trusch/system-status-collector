package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

var configpath = flag.String("c", "/etc/system-status-collector.json", "config file")

// UnitStatus represents one systemd unit
type UnitStatus struct {
	Running bool   `json:"running"`
	Name    string `json:"name"`
	Status  string `json:"status"`
}

// Status represents the state of a system
type Status struct {
	Timestamp  int64 `json:"timestamp"`
	Running    bool
	Error      string
	Uptime     string       `json:"uptime"`
	FileSystem string       `json:"filesystem"`
	Memory     string       `json:"memory"`
	Units      []UnitStatus `json:"units"`
}

// HostConfig is the config for one host
type HostConfig struct {
	Address string
	Units   []string
}

// Config is a status-collector config file
type Config map[string]*HostConfig

// Load loads a configfile
func (cfg *Config) Load(filepath string) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(f)
	return decoder.Decode(cfg)
}

// GetStatus returns the system status to a HostConfig
func GetStatus(config *HostConfig) *Status {
	command := `"\
  uptime && \
  echo ---@@@--- && \
  df -h && \
  echo ---@@@--- && \
  free -h && \
  echo ---@@@--- &&`
	for _, unit := range config.Units {
		command += fmt.Sprintf(`\
    systemctl status %v && \
    echo ---@@@--- && \
    `, unit)
	}
	command += "true;\""
	cmd := exec.Command("ssh", "-t", config.Address, "bash", "-c", command)
	buff := &bytes.Buffer{}
	cmd.Stdout = buff
	err := cmd.Run()
	parts := strings.Split(buff.String(), "---@@@---\n")
	if len(parts) < 3 {
		if err == nil {
			err = errors.New("malformed response: " + buff.String())
		}
		return &Status{
			Timestamp: time.Now().Unix(),
			Error:     err.Error(),
		}
	}
	status := &Status{
		Timestamp:  time.Now().Unix(),
		Running:    true,
		Uptime:     parts[0],
		FileSystem: parts[1],
		Memory:     parts[2],
	}
	for idx := 3; idx < len(parts)-1; idx++ {
		status.Units = append(status.Units, UnitStatus{true, config.Units[idx-3], parts[idx]})
	}
	return status
}

func main() {
	flag.Parse()
	cfg := Config{}
	err := cfg.Load(*configpath)
	if err != nil {
		log.Fatal(err)
	}
	for device, config := range cfg {
		status := GetStatus(config)
		fmt.Println("---------------------", device, "----------------------")
		bs, _ := yaml.Marshal(status)
		fmt.Println(string(bs))
	}
}