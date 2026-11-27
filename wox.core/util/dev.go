package util

const DefaultDevServerPort = 34987

// ProdEnv lets untagged builds and tests opt into production behavior.
var ProdEnv string

func IsProd() bool {
	return ReleaseBuild || ProdEnv == "true"
}

func IsDev() bool {
	return !IsProd()
}
