package gui

// directoryRequests is owned by the UI event loop. Worker results are applied
// only through the same loop, after checking that no path edit superseded them.
type directoryRequests struct{ generation uint64 }

func (r *directoryRequests) invalidate()   { r.generation++ }
func (r *directoryRequests) begin() uint64 { r.generation++; return r.generation }
func (r *directoryRequests) apply(request uint64, update func()) bool {
	if request != r.generation {
		return false
	}
	update()
	return true
}
