/*
Copyright 2023 cuisongliu@qq.com.

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

package apply

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/labring/sealos/pkg/apply/processor"

	"github.com/spf13/cobra"

	"github.com/labring/sealos/pkg/apply/applydrivers"
	"github.com/labring/sealos/pkg/clusterfile"
	"github.com/labring/sealos/pkg/constants"
)

func NewApplierFromFile(cmd *cobra.Command, path string, args *Args) (applydrivers.Interface, error) {
	if !filepath.IsAbs(path) {
		pa, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(pa, path)
	}
	Clusterfile := clusterfile.NewClusterFile(path,
		clusterfile.WithCustomValues(args.Values),
		clusterfile.WithCustomSets(args.Sets),
		clusterfile.WithCustomEnvs(args.CustomEnv),
		clusterfile.WithCustomConfigFiles(args.CustomConfigFiles),
	)
	if err := Clusterfile.Process(); err != nil {
		return nil, err
	}
	cluster := Clusterfile.GetCluster()
	if cluster.Name == "" {
		return nil, fmt.Errorf("cluster name cannot be empty, make sure %s file is correct", path)
	}

	if err := CheckAndInitialize(cluster); err != nil {
		return nil, err
	}

	localpath := constants.Clusterfile(cluster.Name)
	cf := clusterfile.NewClusterFile(localpath)
	err := cf.Process()
	if err != nil && err != clusterfile.ErrClusterFileNotExists {
		return nil, err
	}
	currentCluster := cf.GetCluster()

	ctx := withCommonContext(cmd.Context(), cmd)

	return &applydrivers.Applier{
		Context:        ctx,
		ClusterDesired: cluster,
		ClusterFile:    Clusterfile,
		ClusterCurrent: currentCluster,
		RunNewImages:   GetNewImages(currentCluster, cluster),
	}, nil
}

func Restorer(cmd *cobra.Command, args *RunArgs, image string) error {
	ctx := withCommonContext(cmd.Context(), cmd)

	cf := clusterfile.NewClusterFile(constants.Clusterfile(args.ClusterName))
	err := cf.Process()
	if err != nil && err != clusterfile.ErrClusterFileNotExists {
		return fmt.Errorf("failed to process clusterfile: %w", err)
	}
	restorer, err := processor.NewRestoreProcessor(ctx, args.ClusterName, cf)
	if err != nil {
		return fmt.Errorf("failed to create restore processor: %w", err)
	}
	cluster := cf.GetCluster()

	cluster.Spec.Image = []string{image}
	return restorer.Restore(cf.GetCluster())
}
