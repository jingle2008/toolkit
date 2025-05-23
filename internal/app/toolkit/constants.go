package toolkit

type EditTarget int

const (
	None EditTarget = iota
	Filter
	Alias
)

type StatusMode int

const (
	Edit StatusMode = iota
	Normal
)
