package testing

// RegisterDefaultSuites registers all standard test suites with the provided registry.
func RegisterDefaultSuites(r *Registry) {
	r.Register(&SmokeSuite{})
	r.Register(&CRUDSuite{})
	r.Register(&ScanSuite{})
	r.Register(&BatchSuite{})
	r.Register(&ConcurrencySuite{})
	r.Register(&IntegritySuite{})
	r.Register(&StressSuite{})
}
