package producer

type Producer interface {
	AnalyzeEvent(func(event string) error) error
}
