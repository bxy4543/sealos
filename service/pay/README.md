
# Pay Service

## API

appID sign为提前设置好的，用于验证请求的合法性

### 创建支付会话

**接口 GET v1alpha1/pay/session：**

```shell
curl -X GET -H "Content-Type: application/json" -d '{
    "appID": 45141910007488120,
    "sign": "076f82f8e996d7",
    "amount": "1688",
    "currency": "CNY",
    "user": "jiahui",
    "payMethod": "stripe"
}' http://localhost:2303/v1alpha1/pay/session
```

```shell
{"amount":"1688","currency":"CNY","message":"get stripe sessionID success","orderID":"iGP0mHMJfxfamCBqeA","sessionID":"cs_test_a1UH60aWlJpnbi6c1LA287ymXv0mWDYdT6oonpRduTs9zzSF6OU87WSXa2","user":"jiahui"}
```

### 查询支付状态
GET v1alpha1/pay/status


```shell
	PaymentNotPaid    = "notpaid"
	PaymentProcessing = "processing"
	PaymentFailed     = "failed"
	PaymentExpired    = "expired"
	PaymentSuccess    = "success"
	PaymentUnknown    = "unknown"
```

```shell
curl -X GET -H "Content-Type: application/json" -d '{
    "appID": 45141910007488120,
    "sign": "076f82f8e996d7",
    "orderID": "iGP0mHMJfxfamCBqeA",
    "payMethod": "stripe",
    "user": "jiahui",
    "sessionID": "cs_test_a1UH60aWlJpnbi6c1LA287ymXv0mWDYdT6oonpRduTs9zzSF6OU87WSXa2"
}'  http://localhost:2303/v1alpha1/pay/status

{"message":"payment status is: notpaid,please try again later","orderID":"iGP0mHMJfxfamCBqeA","status":"notpaid"}
```


### 查询历史账单信息

```shell
{
    "appID": 45141910007488120,
    "sign": "076f82f8e996d7",
    "user": "jiahui"
}
```

```shell
curl -X GET -H "Content-Type: application/json" -d '{
    "appID": 45141910007488120,
    "sign": "076f82f8e996d7",
    "user": "jiahui"
}' http://localhost:2303/v1alpha1/pay/bill
```