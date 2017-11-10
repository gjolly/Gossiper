package tools

import "fmt"

type RumorMessage struct {
	Origin   string
	ID       uint32
	Text     string
	Dest     string
	HopLimit uint32
}

func (m RumorMessage) String() string {
	return "(" + m.Origin + ", " + fmt.Sprint(m.ID) + ", " + m.Text + ")"
}

func (m RumorMessage) IsPrivate() bool {
	return m.Dest != ""
}
