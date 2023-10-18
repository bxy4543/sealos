package main

import (
	"context"
	"fmt"
	"os"

	"go.mongodb.org/mongo-driver/mongo/options"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

/*
_id(ObjectId)	payMethod(String)	currency (String)	amountOption(Array)	exchangeRate(decimal128)	taxRate(decimal128)
	stripe	USD	"168","388","768","1068","2268"	2	0.5
	wechat	CNY	"258","468","862","1218","1822"	1	0.08
*/

type PayMethod struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	PayMethod    string             `bson:"payMethod,omitempty"`
	Currency     string             `bson:"currency,omitempty"`
	AmountOption []string           `bson:"amountOption,omitempty"`
	ExchangeRate float64            `bson:"exchangeRate,omitempty"`
	TaxRate      float64            `bson:"taxRate,omitempty"`
}

/*
_id（ObjectId）	appID (Int64)	sign(String)	payAppName(String)	methods(Array)
	45141910007488120	076f82f8e996d7	sealos	wechat、stripe
	59168566064815128	f19c993d88876d	laf.io	wechat、stripe
*/

type App struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	AppID      int64              `bson:"appID,omitempty"`
	Sign       string             `bson:"sign,omitempty"`
	Region     string             `bson:"region,omitempty"`
	PayAppName string             `bson:"payAppName,omitempty"`
	Methods    []string           `bson:"methods,omitempty"`
}

/*
_id（ObjectId）	orderID（String）	user（String）	amount（string）	currency（String）	payTime（String）	payMethod（String）	appID (int64)	status（String）
	RHKZfE-pc1MsXkX2ow	xy	1688	CNY	2023/9/7 18:04	wechat	59168566064815128	notpaid
	gXUOMpg2SCoBHUyIbU	xy	1288	CNY	2023/9/7 18:05	wechat	59168566064815128	notpaid
	NaTSOUjF-k3GMceEnE	xy	1288	USD	2023/9/7 18:05	stripe	59168566064815128	expired
*/

type PaymentDetail struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	OrderID   string             `bson:"orderID,omitempty"`
	User      string             `bson:"user,omitempty"`
	Amount    string             `bson:"amount,omitempty"`
	Currency  string             `bson:"currency,omitempty"`
	PayTime   string             `bson:"payTime,omitempty"`
	PayMethod string             `bson:"payMethod,omitempty"`
	AppID     int64              `bson:"appID,omitempty"`
	Status    string             `bson:"status,omitempty"`
}

/*
_id（ObjectId）	orderID（String）	user（String）	amount（int64）	payTime (String)	payMethod (String)	appID (Int64)	details (Object)
	RHKZfE-pc1MsXkX2ow	xy	1688	2023/9/7 18:04	wechat	59168566064815128	{"tradNo":“049dfbf0b96ae9e2fa54a4b8eed6ea34”,"codeURL":"weixin://wxpay/bizpayurl?pr=ydG9IQ4zz"}
	gXUOMpg2SCoBHUyIbU	xy	1288	2023/9/7 18:05	wechat	59168566064815128	{"tradNo":“db27af04c65bd27bb3c3708addbafc01”,"codeURL":"weixin://wxpay/bizpayurl?pr=BVDUf35zz"}
	NaTSOUjF-k3GMceEnE	xy	2576	2023/9/7 18:05	stripe	59168566064815128	 {"sessionID":"cs_test_a14Mf2i1hY1i5SkvJEsndrnklzgDuXBGyTI59csIbSCgZ0RAbPjIjMyzeR"}
*/

type OrderDetail struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	OrderID   string             `bson:"orderID,omitempty"`
	User      string             `bson:"user,omitempty"`
	Amount    int64              `bson:"amount,omitempty"`
	PayTime   string             `bson:"payTime,omitempty"`
	PayMethod string             `bson:"payMethod,omitempty"`
	AppID     int64              `bson:"appID,omitempty"`
	//Details   Details            `bson:"details,omitempty"`
}

func CreateApp(client mongo.Client, apps []App) error {
	for i := range apps {
		_, err := client.Database("sealos-resources").Collection("app").InsertOne(context.Background(), apps[i])
		if err != nil {
			return fmt.Errorf("insert app %s failed: %v", apps[i].PayAppName, err)
		}
		fmt.Println("insert app", apps[i].PayAppName, "success")
	}
	return nil
}

func CreatePayMethod(client mongo.Client, payMethods []PayMethod) error {
	for i := range payMethods {
		_, err := client.Database("sealos-resources").Collection("payMethod").InsertOne(context.Background(), payMethods[i])
		if err != nil {
			return fmt.Errorf("insert payMethod %s failed: %v", payMethods[i].PayMethod, err)
		}
		fmt.Println("insert payMethod", payMethods[i].PayMethod, "success")
	}
	return nil
}

func main() {
	/*
	   _id（ObjectId）	appID (Int64)	sign(String)	payAppName(String)	methods(Array)
	   	45141910007488120	076f82f8e996d7	sealos	wechat、stripe
	   	59168566064815128	f19c993d88876d	laf.io	wechat、stripe
	*/
	// new mongo client
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		fmt.Println("connect to mongo failed:", err)
		os.Exit(1)
	}
	//err = CreateApp(*client, []App{
	//	{
	//		AppID:      45141910007488120,
	//		Sign:       "076f82f8e996d7",
	//		PayAppName: "sealos",
	//		Region:     "io",
	//		Methods:    []string{"wechat", "stripe"},
	//	},
	//})
	err = CreatePayMethod(*client, []PayMethod{
		{
			PayMethod:    "stripe",
			Currency:     "USD",
			AmountOption: []string{"168", "388", "768", "1068", "2268"},
			ExchangeRate: 2,
			TaxRate:      0.5,
		},
		{
			PayMethod: "wechat",
			Currency:  "CNY",
			AmountOption: []string{
				"258", "468", "862", "1218", "1822",
			},
			ExchangeRate: 1,
			TaxRate:      0.08,
		},
	})
	if err != nil {
		fmt.Println("create app failed:", err)
		os.Exit(1)
	}
}
