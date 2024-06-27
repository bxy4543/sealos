// Copyright © 2022 cuisongliu@qq.com.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/labring/sealos/pkg/apply"
	"github.com/labring/sealos/pkg/utils/logger"
)

const exampleRestore = `
restore image :
	sealos restore image kubernetes:v1.27.8
`

// addCmd represents the add command
func newRestoreCmd() *cobra.Command {
	restoreArgs := &apply.RunArgs{
		Cluster: &apply.Cluster{},
		SSH:     &apply.SSH{},
	}
	var restoreCmd = &cobra.Command{
		Use:     "restore",
		Short:   "restore cluster",
		Args:    cobra.ExactArgs(1),
		Example: exampleRestore,
		RunE: func(cmd *cobra.Command, args []string) error {
			return apply.Restorer(cmd, restoreArgs, args[0])
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("image can't be empty")
			}
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			logger.Info(getContact())
		},
	}
	setRequireBuildahAnnotation(restoreCmd)
	restoreArgs.RegisterFlags(restoreCmd.Flags())
	return restoreCmd
}
