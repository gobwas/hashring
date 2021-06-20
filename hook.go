package hashring

//go:generate gtrace -tag hashring_trace

//gtrace:gen
type traceRing struct {
	OnInsert    func(*point) traceRingInsert
	OnDelete    func(*point) traceRingDelete
	OnFix       func(*point) traceRingFix
	OnFixNeeded func(*point)
}

//gtrace:gen
type traceRingInsert struct {
	OnDone      func(bool)
	OnCollision func(*point)
}

//gtrace:gen
type traceRingDelete struct {
	OnDone        func(bool)
	OnProcessing  func(*point) func()
	OnTwinDelete  func(p *point)
	OnTwinRestore func(p *point)
}

//gtrace:gen
type traceRingFix struct {
	OnDone func()
}
