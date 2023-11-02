package prometheus

type Interface interface {
	QueryLvmVgsTotalFree(QueryParams) (float64, error)
}
