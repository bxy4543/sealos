// Copyright © 2023 sealos.
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

package prometheus

import (
	"fmt"

	"github.com/prometheus/common/model"
)

type prometheus struct {
	executor QueryExecutor
}

func (p *prometheus) QueryLvmVgsTotalFree(queryParams QueryParams) (float64, error) {
	matrixData, err := p.queryLvmVgsTotalFree(queryParams)
	if err != nil {
		return 0, err
	}

	// Assuming the value you want is the first value of the first series.
	firstValue := matrixData[0].Value
	return float64(firstValue), nil
}

func (p *prometheus) queryLvmVgsTotalFree(queryParams QueryParams) (model.Vector, error) {
	if queryParams.Node == "" {
		return nil, fmt.Errorf("node can not be empty")
	}
	result, err := p.executor.Execute(queryParams)
	if err != nil {
		return nil, err
	}
	matrixData, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("invalid result type: %T", result)
	}
	if len(matrixData) == 0 {
		return nil, fmt.Errorf("query not found data")
	}
	return matrixData, nil
}

func NewPrometheus(address string) (Interface, error) {
	exe, err := NewQueryExecutor(address)
	if err != nil {
		return nil, err
	}

	return &prometheus{
		executor: exe,
	}, nil
}
