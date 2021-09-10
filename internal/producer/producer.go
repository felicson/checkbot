package producer

type Matchers map[string][]string
type Searcher interface {
	SearchByPattern(string) (Matchers, error)
}
