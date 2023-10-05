package plugin

type Plugin interface {
	Init(initParams InitParams)
	Query(query Query) []QueryResult
}

type InitParams struct {
	API API
}
