package parser

import "runtime/debug"

var Version = "v1.0.0"

func GetVersion() string {
	if Version != "" && Version != "v1.0.0" {
		return Version
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}

		var revision, modified string
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value
			}
		}

		if revision != "" {
			if len(revision) > 7 {
				revision = revision[:7]
			}
			if modified == "true" {
				return revision + "-dirty"
			}
			return revision
		}
	}

	return Version
}
