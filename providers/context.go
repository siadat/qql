package providers

// OrderTerm mirrors parser.OrderTerm in a serializable form so providers can
// receive an ORDER BY hint without importing the parser package.
type OrderTerm struct {
	Col  string `json:"col"`
	Desc bool   `json:"desc,omitempty"`
}

// Context carries everything a provider may want to know about the current
// query. Built-in providers only look at Source and Prefix; external providers
// receive the full bundle as a hint payload (qql still re-applies WHERE and
// ORDER BY to whatever rows the provider returns, so providers are free to
// ignore any field they don't understand).
type Context struct {
	Source   string
	Files    []string
	Prefix   string
	Provider string
	Select   []string
	Where    string
	OrderBy  []OrderTerm
}
