// Copyright © 2024 sealos.
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

package cockroach

import (
	"os"
	"testing"

	"github.com/labring/sealos/controllers/pkg/types"
)

type TestConfig struct {
	RegionID      string
	V2GlobalDBURI string
	V2LocalDBURI  string
}

var testConfig = TestConfig{}

func TestCockroach_GetUserOauthProvider(t *testing.T) {
	os.Setenv("LOCAL_REGION", testConfig.RegionID)
	ck, err := NewCockRoach(testConfig.V2GlobalDBURI, testConfig.V2LocalDBURI)
	if err != nil {
		t.Errorf("NewCockRoach() error = %v", err)
		return
	}
	defer ck.Close()

	provider, err := ck.GetUserOauthProvider(&types.UserQueryOpts{
		Owner: "xxx",
	})
	if err != nil {
		t.Errorf("GetUserOauthProvider() error = %v", err)
		return
	}
	t.Logf("provider: %+v", provider)
}

func TestCockroach_InviteRewardHandler(t *testing.T) {
	os.Setenv("LOCAL_REGION", "")
	ck, err := NewCockRoach("", "")
	if err != nil {
		t.Errorf("NewCockRoach() error = %v", err)
		return
	}
	defer ck.Close()

	amount, err := ck.InviteRewardHandler("eWpJlOG_90", []string{"c2e33790-bbfd-417c-9e00-3389725a738f", "da888a48-470f-49f2-8fc0-6f47cb5048c1", "da888a48-470f-49f2-8fc0-6f47cb5048c1"}, 0.1)
	if err != nil {
		t.Errorf("InviteRewardHandler2() error = %v", err)
		return
	}
	t.Logf("amount: %v", amount)
}
