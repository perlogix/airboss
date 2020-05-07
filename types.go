package airboss

// File type
type File struct {
	Path string
	Fd   uint64
}

// Metrics type
type Metrics struct {
	CPU     float64
	Memory  float64
	Threads int32
}
