package glfw

type mainEvent uint8

const (
	mainStop mainEvent = iota
	mainFunction
	mainFrame
)

type funcData struct {
	f    func()
	done chan struct{} // Zero allocation signalling channel
}
