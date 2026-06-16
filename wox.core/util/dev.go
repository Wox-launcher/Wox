package util

const DefaultDevServerPort = 34987

// will set by -X flag on build
var ProdEnv string

func IsProd() bool {
	return ProdEnv == "true"
}

func IsDev() bool {
	return !IsProd()
}
