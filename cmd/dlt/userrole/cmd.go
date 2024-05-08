/*
Copyright (c) 2022 Red Hat, Inc.

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

package userrole

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	unlinkuserrole "github.com/openshift/rosa/cmd/unlink/userrole"
	"github.com/openshift/rosa/pkg/arguments"
	"github.com/openshift/rosa/pkg/aws"
	awscb "github.com/openshift/rosa/pkg/aws/commandbuilder"
	"github.com/openshift/rosa/pkg/helper"
	"github.com/openshift/rosa/pkg/interactive"
	"github.com/openshift/rosa/pkg/interactive/confirm"
	"github.com/openshift/rosa/pkg/rosa"
)

var args struct {
	roleARN string
}

var Cmd = &cobra.Command{
	Use:     "user-role",
	Aliases: []string{"userrole"},
	Short:   "Delete user role",
	Long:    "Delete user role from the current AWS account",
	Example: ` # Delete user role
rosa delete user-role --role-arn {prefix}-User-{username}-Role`,
	Run:  run,
	Args: cobra.MaximumNArgs(1),
}

func init() {
	flags := Cmd.Flags()

	flags.StringVar(
		&args.roleARN,
		"role-arn",
		"",
		"Role ARN to delete from the user role from the AWS account")

	interactive.AddModeFlag(Cmd)

	confirm.AddFlag(flags)
	interactive.AddFlag(flags)
}

func run(cmd *cobra.Command, argv []string) {
	r := rosa.NewRuntime().WithAWS().WithOCM()
	defer r.Cleanup()

	mode, err := interactive.GetMode()
	if err != nil {
		r.Reporter.Errorf("%s", err)
		os.Exit(1)
	}

	if len(argv) > 0 {
		args.roleARN = argv[0]
	}

	// Determine if interactive mode is needed
	if !interactive.Enabled() && (!cmd.Flags().Changed("mode")) {
		interactive.Enable()
	}

	if r.Reporter.IsTerminal() {
		r.Reporter.Infof("Deleting user role")
	}

	roleARN := args.roleARN

	if !interactive.Enabled() && roleARN == "" {
		interactive.Enable()
	}

	if interactive.Enabled() {
		roleARN, err = interactive.GetString(interactive.Input{
			Question: "User Role ARN",
			Help:     cmd.Flags().Lookup("role-arn").Usage,
			Default:  roleARN,
			Required: true,
			Validators: []interactive.Validator{
				aws.ARNValidator,
			},
		})
		if err != nil {
			r.Reporter.Errorf("Expected a valid user role ARN to delete from the current AWS account: %s", err)
			os.Exit(1)
		}
	}

	err = aws.ARNValidator(roleARN)
	if err != nil {
		r.Reporter.Errorf("Expected a valid user role ARN to delete from the current AWS account: %s", err)
		os.Exit(1)
	}

	err = r.AWSClient.ValidateRoleARNAccountIDMatchCallerAccountID(roleARN)
	if err != nil {
		r.Reporter.Errorf("%s", err)
		os.Exit(1)
	}

	if !confirm.Prompt(true, "Delete the '%s' role from the AWS account?", roleARN) {
		os.Exit(0)
	}

	currentAccount, err := r.OCMClient.GetCurrentAccount()
	if err != nil {
		r.Reporter.Errorf("Error getting current account: %v", err)
		os.Exit(1)
	}

	linkedRoles, err := r.OCMClient.GetAccountLinkedUserRoles(currentAccount.ID())
	if err != nil {
		r.Reporter.Errorf("An error occurred while trying to get the account linked roles")
		os.Exit(1)
	}
	isLinked := helper.Contains(linkedRoles, roleARN)

	if interactive.Enabled() && !cmd.Flags().Changed("mode") {
		mode, err = interactive.GetOptionMode(cmd, mode, "User role deletion mode")
		if err != nil {
			r.Reporter.Errorf("Expected a valid role deletion mode: %s", err)
			os.Exit(1)
		}
	}

	roleName, err := aws.GetResourceIdFromARN(roleARN)
	if err != nil {
		r.Reporter.Errorf("%s", err)
		os.Exit(1)
	}

	roleExistOnAWS, existingRoleARN, err := r.AWSClient.CheckRoleExists(roleName)
	if err != nil {
		r.Reporter.Errorf("%v", err)
	}
	if !roleExistOnAWS {
		r.Reporter.Warnf("the ARN %s does not exist. Nothing to delete", roleARN)
	} else if existingRoleARN != roleARN {
		r.Reporter.Warnf("role with same name but different ARN exists. Existing role ARN: %s", existingRoleARN)
		os.Exit(1)
	}

	isUserRole, err := r.AWSClient.IsUserRole(&roleName)
	if err != nil {
		r.Reporter.Errorf("%s", err)
		os.Exit(1)
	}
	if !isUserRole {
		r.Reporter.Errorf("Role '%s' is not a user role", roleName)
		os.Exit(1)
	}

	switch mode {
	case interactive.ModeAuto:
		r.OCMClient.LogEvent("ROSADeleteUserMRoleModeAuto", nil)
		if isLinked {
			r.Reporter.Warnf("Role ARN '%s' is linked to account '%s'",
				roleARN, currentAccount.ID())
			arguments.DisableRegionDeprecationWarning = true // disable region deprecation warning
			unlinkuserrole.Cmd.Run(unlinkuserrole.Cmd, []string{roleARN})
			arguments.DisableRegionDeprecationWarning = false // enable region deprecation again
		}
		err := r.AWSClient.DeleteUserRole(roleName)
		if err != nil {
			r.Reporter.Errorf("There was an error deleting the user role: %s", err)
			os.Exit(1)
		}
		r.Reporter.Infof("Successfully deleted the user role")
	case interactive.ModeManual:
		r.OCMClient.LogEvent("ROSADeleteUserMRoleModeManual", nil)
		commands, err := buildCommands(roleName, roleARN, isLinked, r.AWSClient)
		if err != nil {
			r.Reporter.Errorf("%s", err)
			os.Exit(1)
		}
		if r.Reporter.IsTerminal() {
			r.Reporter.Infof("Run the following commands to delete the user role:\n")
		}
		fmt.Println(commands)
	default:
		r.Reporter.Errorf("Invalid mode. Allowed values are %s", interactive.Modes)
		os.Exit(1)
	}
}

func buildCommands(roleName string, roleARN string, isLinked bool, awsClient aws.Client) (string, error) {
	var commands []string
	if isLinked {
		unlinkRole := fmt.Sprintf("rosa unlink user-role \\\n"+
			"\t--role-arn %s", roleARN)
		commands = append(commands, unlinkRole)
	}

	policies, err := awsClient.GetAttachedPolicy(&roleName)
	if err != nil {
		return "", err
	}
	for _, policy := range policies {
		detachPolicy := awscb.NewIAMCommandBuilder().
			SetCommand(awscb.DetachRolePolicy).
			AddParam(awscb.RoleName, roleName).
			AddParam(awscb.PolicyArn, policy.PolicyArn).
			Build()
		commands = append(commands, detachPolicy)
	}

	hasPermissionBoundary, err := awsClient.HasPermissionsBoundary(roleName)
	if err != nil {
		return "", err
	}
	if hasPermissionBoundary {
		deletePermissionBoundary := awscb.NewIAMCommandBuilder().
			SetCommand(awscb.DeleteRolePermissionsBoundary).
			AddParam(awscb.RoleName, roleName).
			Build()
		commands = append(commands, deletePermissionBoundary)
	}

	deleteRole := awscb.NewIAMCommandBuilder().
		SetCommand(awscb.DeleteRole).
		AddParam(awscb.RoleName, roleName).
		Build()
	commands = append(commands, deleteRole)

	return awscb.JoinCommands(commands), nil
}
