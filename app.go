package main

// AppService is the main service exposed to the Wails frontend.
// Methods on this struct are callable from JavaScript via Wails bindings.
type AppService struct{}

// Greet returns a greeting for the given name.
// This is a placeholder — real methods will be added in Phase 3.
func (a *AppService) Greet(name string) string {
	return "Hello, " + name + "!"
}
