# Dependency maintenance

The checked-in modules build and pass tests with Go 1.26.4. Direct dependencies are labeled explicitly. `go mod tidy` could not finish because downloads of upstream test-only dependencies (`modernc.org/sqlite` and `golang.org/x/tools`) stalled. Run it on a working network before the first release; no checksum verification was disabled.
