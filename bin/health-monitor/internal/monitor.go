package internal

type Monitor interface {
	GetName() string
	SetName(string)
	Monitor() error
	Validate() error
	ExportStats()
}
