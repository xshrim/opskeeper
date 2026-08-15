package version

var (
	Value     = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

func Current() Info {
	return Info{
		Version:   Value,
		Commit:    Commit,
		BuildTime: BuildTime,
	}
}

func LogAttributes() []any {
	return []any{
		"version", Value,
		"commit", Commit,
		"build_time", BuildTime,
	}
}
