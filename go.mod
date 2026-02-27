module kian

go 1.25

require (
	github.com/energye/systray v1.0.3
	github.com/ggerganov/whisper.cpp/bindings/go v0.0.0
	github.com/gordonklaus/portaudio v0.0.0-20230709114228-aafa478834f5
	github.com/stretchr/testify v1.11.1
	golang.org/x/sync v0.11.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/ggerganov/whisper.cpp/bindings/go => ./third_party/whisper.cpp/bindings/go
