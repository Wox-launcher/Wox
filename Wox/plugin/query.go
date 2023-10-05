package plugin

// Query from Wox. See "Doc/Query.md" for details.
type Query struct {
	// Raw query, this includes trigger keyword if it has.
	// We didn't recommend use this property directly. You should always use Search property.
	RawQuery string

	// Trigger keyword of a query. It can be empty if user is using global trigger keyword.
	// Empty trigger keyword means this query will be a global query.
	TriggerKeyword string

	// Command part of a query.
	// Empty command means this query doesn't have a command.
	Command string

	// Search part of a query.
	// Empty search means this query doesn't have a search part.
	Search string
}

type QueryResult struct {
	// Result id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
	Id       string
	Title    string
	SubTitle string
	Icon     string
	Score    int
	Action   func() bool
}
