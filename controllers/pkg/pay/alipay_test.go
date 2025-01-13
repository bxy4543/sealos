package pay

import (
	"testing"
	"time"
)

func TestAntom_CreatePayment(t *testing.T) {
	var alipay AliPay
	tradeNO, codeURL, err := alipay.CreatePayment(10000, "sealos", "ceshi")
	if err != nil {
		t.Fatal("CreatePayment error:", err)
	}
	t.Log("tradeNO:", tradeNO, "codeURL:", codeURL)

	time.Sleep(1 * time.Minute)

	status, amount, metadata, err := alipay.GetPaymentDetails(tradeNO)
	if err != nil {
		t.Fatal("GetPaymentDetails error:", err)
	}
	t.Log("status:", status, "amount:", amount)
	t.Log("metadata:", metadata)

	if err = alipay.ExpireSession(tradeNO); err != nil {
		t.Fatal("ExpireSession error:", err)
	}
}
