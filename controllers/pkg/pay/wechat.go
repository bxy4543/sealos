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

package pay

import (
	"encoding/json"
	"fmt"
)

func (w WechatPayment) CreatePayment(amount int64, user, describe string) (string, string, error) {
	tradeNO := GetRandomString(32)
	codeURL, err := WechatPay(amount, user, tradeNO, describe, "")
	if err != nil {
		return "", "", err
	}
	return tradeNO, codeURL, nil
}

func (w WechatPayment) GetPaymentDetails(sessionID string) (status string, amount int64, metadata string, err error) {
	orderResp, err := QueryOrder(sessionID)
	if err != nil {
		return "", 0, "", err
	}
	switch *orderResp.TradeState {
	case StatusSuccess:
		amount = *orderResp.Amount.Total
		metadataRaw, err := json.Marshal(orderResp)
		if err != nil {
			return "", 0, "", fmt.Errorf("marshal metadata session failed: %s", err.Error())
		}
		metadata = string(metadataRaw)
		return PaymentSuccess, amount, metadata, nil
	case StatusProcessing:
		return PaymentProcessing, amount, metadata, nil
	case StatusNotPay:
		return PaymentNotPaid, amount, metadata, nil
	case StatusFail:
		return PaymentFailed, amount, metadata, fmt.Errorf("order failed: %s", *orderResp.TradeStateDesc)
	default:
		return PaymentUnknown, amount, metadata, fmt.Errorf("unknown order status: %s", *orderResp.TradeState)
	}
}

func (w WechatPayment) ExpireSession(_ string) error {
	return nil
}
