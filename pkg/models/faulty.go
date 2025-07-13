package models

// Faulty marks an item that can be checked for faults.
type Faulty interface {
	IsFaulty() bool
}
