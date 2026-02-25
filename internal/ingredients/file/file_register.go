package file

func init() {
	provMap = make(map[string]FileProvider)
}

// RegisterFileProviders registers the built-in file providers.
// This must be called after init() to register providers from sub-packages.
func RegisterFileProviders() {
	// Providers are registered via their own init() functions
	// or explicitly by the caller.
}
