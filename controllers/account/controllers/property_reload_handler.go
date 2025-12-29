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

package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/labring/sealos/controllers/pkg/database"
	"github.com/labring/sealos/controllers/pkg/resources"
	ctrl "sigs.k8s.io/controller-runtime"
)

var reloadLogger = ctrl.Log.WithName("property-reload-handler")

// PropertyReloadHandler is an HTTP handler to reload property types from database
type PropertyReloadHandler struct {
	DBClient database.Interface
}

func (h *PropertyReloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reloadLogger.Info("received request to reload property types")

	// Reload property types from database
	if err := h.DBClient.ReloadPropertyTypeLS(); err != nil {
		reloadLogger.Error(err, "failed to reload property types")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the reloaded properties count
	propertyCount := 0
	if resources.DefaultPropertyTypeLS != nil {
		propertyCount = len(resources.DefaultPropertyTypeLS.Types)
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Property types reloaded successfully",
		"count":   propertyCount,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		reloadLogger.Error(err, "failed to encode response")
	}
}
