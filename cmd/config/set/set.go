/*
Copyright (c) 2024 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package set

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/openshift/rosa/pkg/config"
	"github.com/openshift/rosa/pkg/rosa"
)

var args struct {
	debug bool
}

var Cmd = NewConfigCommand()

func NewConfigCommand() *cobra.Command {
	Cmd := &cobra.Command{
		Use:   "set [flags] VARIABLE VALUE",
		Short: "Sets the variable's value",
		Long:  "Sets the value of a config variable. See 'rosa config --help' for supported config variables.",
		Args:  cobra.ExactArgs(2),
		Run:   run,
	}
	flags := Cmd.Flags()
	flags.BoolVar(
		&args.debug,
		"debug",
		false,
		"Enable debug mode.",
	)
	return Cmd
}

func run(cmd *cobra.Command, argv []string) {
	r := rosa.NewRuntime()

	err := SaveConfig(argv[0], argv[1])
	if err != nil {
		r.Reporter.Errorf(err.Error())
		os.Exit(1)
	}
}

func SaveConfig(arg, value string) error {
	// Load the configuration:
	cfg, err := config.Load()
	if err != nil {
		err := fmt.Errorf("Config file doesn't exist yet")
		return err
	}

	// Create an empty configuration if the configuration file doesn't exist:
	if cfg == nil {
		cfg = &config.Config{}
	}

	// Copy the value given in the command line to the configuration:
	switch arg {
	case "access_token":
		cfg.AccessToken = value
	case "client_id":
		cfg.ClientID = value
	case "client_secret":
		cfg.ClientSecret = value
	case "insecure":
		cfg.Insecure, err = strconv.ParseBool(value)
		if err != nil {
			err := fmt.Errorf("Failed to set insecure: %v", value)
			return err
		}
	case "refresh_token":
		cfg.RefreshToken = value
	case "scopes":
		err := errors.New("Setting scopes is unsupported")
		return err
	case "token_url":
		cfg.TokenURL = value
	case "url":
		cfg.URL = value
	case "fedramp":
		cfg.FedRAMP, err = strconv.ParseBool(value)
		if err != nil {
			err := fmt.Errorf("Failed to set fedramp: %v", value)
			return err
		}
	default:
		err := errors.New("Unknown setting")
		return err
	}

	// Save the configuration:
	err = config.Save(cfg)
	if err != nil {
		err := fmt.Errorf("Can't save config file: %v", err)
		return err
	}

	return nil
}