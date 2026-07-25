module github.com/suglasp/max-go

go 1.26.5

require (
	github.com/hajimehoshi/ebiten/v2 v2.9.9
	golang.org/x/sys v0.46.0
	golang.org/x/term v0.44.0
	miniaudio v0.0.0
	soloud v0.0.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20250923094054-ea854a63cce1 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.4.0 // indirect
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	golang.org/x/sync v0.17.0 // indirect
)

replace soloud => ./dependencies/soloud-go

replace miniaudio => ./dependencies/miniaudio-go
