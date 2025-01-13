package pay

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/labring/sealos/controllers/pkg/utils/env"

	defaultAlipayClient "github.com/alipay/global-open-sdk-go/com/alipay/api"
	"github.com/alipay/global-open-sdk-go/com/alipay/api/model"
	"github.com/alipay/global-open-sdk-go/com/alipay/api/request/pay"
	responsePay "github.com/alipay/global-open-sdk-go/com/alipay/api/response/pay"
	"github.com/google/uuid"
)

const (
	envAlipayGateWayURL         = "ALIPAY_GATEWAY_URL"
	envAlipayClientID           = "ALIPAY_CLIENT_ID"
	envAlipayMerchantPrivateKey = "ALIPAY_MERCHANT_PRIVATE_KEY"
	envAlipayAlipayPublicKey    = "ALIPAY_ALIPAY_PUBLIC_KEY"

	envOrderDescription  = "ORDER_DESCRIPTION"
	envAliPayRedirectURL = "ALIPAY_REDIRECT_URL"

	envPayCurrency = "PAY_CURRENCY"
)

type AliPay struct {
}

var alipayGatewayURL, alipayClientID, alipayMerchantPrivateKey, alipayAlipayPublicKey string

func (a *AliPay) CreatePayment(amount int64, user, describe string) (string, string, error) {
	client := defaultAlipayClient.NewDefaultAlipayClient(
		alipayGatewayURL,
		alipayClientID,
		alipayMerchantPrivateKey,
		alipayAlipayPublicKey)
	resp, err := doPay(client, amount, user, describe)
	if err != nil {
		return "", "", err
	}
	return resp.PaymentRequestId, resp.NormalUrl, nil
}

func (a *AliPay) GetPaymentDetails(paymentRequestID string) (status string, amount int64, metadata string, err error) {
	client := defaultAlipayClient.NewDefaultAlipayClient(
		alipayGatewayURL,
		alipayClientID,
		alipayMerchantPrivateKey,
		alipayAlipayPublicKey)
	resp, err := payQuery(paymentRequestID, client)
	if err != nil {
		return "", 0, "", err
	}
	amountValue, err := strconv.ParseInt(resp.PaymentAmount.Value, 10, 64)
	if err != nil {
		return "", 0, "", fmt.Errorf("parse amount %s failed: %s", resp.PaymentAmount.Value, err.Error())
	}

	switch resp.PaymentStatus {
	case model.TransactionStatusType_SUCCESS:
		amount = amountValue
		metadataRaw, err := json.Marshal(resp)
		if err != nil {
			return "", 0, "", fmt.Errorf("marshal metadata session failed: %s", err.Error())
		}
		metadata = string(metadataRaw)
		return PaymentSuccess, amount, metadata, nil
	case model.TransactionStatusType_PROCESSING:
		return PaymentProcessing, amount, metadata, nil
	case model.TransactionStatusType_FAIL, model.TransactionStatusType_CANCELLED, model.TransactionStatusType_PENDING:
		metadata = resp.PaymentResultMessage
		return PaymentFailed, amount, metadata, nil
	default:
		return PaymentUnknown, amount, metadata, fmt.Errorf("unknown order status: %s", resp.PaymentStatus)
	}
}

func (a *AliPay) ExpireSession(paymentRequestID string) error {
	client := defaultAlipayClient.NewDefaultAlipayClient(
		alipayGatewayURL,
		alipayClientID,
		alipayMerchantPrivateKey,
		alipayAlipayPublicKey)
	return cancel(paymentRequestID, client)
}

var defaultAntomPaymentURL = "https://open-sea-global.alipay.com"

func doPay(body *defaultAlipayClient.DefaultAlipayClient, amount int64, user, describe string) (*responsePay.AlipayPayResponse, error) {
	payRequest, request := pay.NewAlipayPayRequest()

	order := &model.Order{}
	if describe == "" {
		describe = "sealos cloud payment"
	}
	order.OrderDescription = env.GetEnvWithDefault(envOrderDescription, fmt.Sprintf(describe+"; order for %s", user))
	order.ReferenceOrderId = uuid.NewString()
	order.OrderAmount = model.NewAmount(strconv.FormatInt(amount, 10), UppercaseCurrency)

	request.PaymentAmount = model.NewAmount(strconv.FormatInt(amount, 10), UppercaseCurrency)
	request.PaymentMethod = &model.PaymentMethod{PaymentMethodType: model.ALIPAY_HK}
	request.PaymentNotifyUrl = "https://www.yourNotifyUrl.com"
	request.PaymentRedirectUrl = localURL + env.GetEnvWithDefault(envAliPayRedirectURL, "?openapp=system-costcenter%3FalipayState%3Dredirect")
	request.PaymentRequestId = uuid.NewString()
	request.ProductCode = model.CASHIER_PAYMENT
	request.SettlementStrategy = &model.SettlementStrategy{
		SettlementCurrency: UppercaseCurrency,
	}
	request.Order = order
	request.Env = &model.Env{TerminalType: model.WEB}
	execute, err := body.Execute(payRequest)
	if err != nil {
		return nil, fmt.Errorf("payment exec failed: %s", err.Error())
	}
	resp := execute.(*responsePay.AlipayPayResponse)
	if resp.Result.ResultStatus != "U" && resp.Result.ResultCode != "PAYMENT_IN_PROCESS" {
		return nil, fmt.Errorf("payment failed, result: %s", resp.Result)
	}
	return execute.(*responsePay.AlipayPayResponse), nil
}

func payQuery(paymentRequestID string, body *defaultAlipayClient.DefaultAlipayClient) (*responsePay.AlipayPayQueryResponse, error) {
	queryRequest := pay.AlipayPayQueryRequest{}
	queryRequest.PaymentRequestId = paymentRequestID
	request := queryRequest.NewRequest()
	execute, err := body.Execute(request)
	if err != nil {
		return nil, fmt.Errorf("query exec failed: %s", err.Error())
	}
	return execute.(*responsePay.AlipayPayQueryResponse), nil
}

func cancel(paymentRequestID string, client *defaultAlipayClient.DefaultAlipayClient) error {
	request, cancelRequest := pay.NewAlipayPayCancelRequest()
	cancelRequest.PaymentRequestId = paymentRequestID
	execute, err := client.Execute(request)
	if err != nil {
		return fmt.Errorf("cancel exec failed: %s", err.Error())
	}
	response := execute.(*responsePay.AlipayPayCancelResponse)

	if response.Result.ResultCode == "SUCCESS" {
		return nil
	}
	return fmt.Errorf("cancel failed: %s", response.Result)
}

func init() {
	alipayGatewayURL = env.GetEnvWithDefault(envAlipayGateWayURL, defaultAntomPaymentURL)
	alipayClientID = os.Getenv(envAlipayClientID)
	alipayMerchantPrivateKey = os.Getenv(envAlipayMerchantPrivateKey)
	alipayAlipayPublicKey = os.Getenv(envAlipayAlipayPublicKey)
}
