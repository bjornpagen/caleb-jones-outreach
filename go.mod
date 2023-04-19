module github.com/bjornpagen/caleb-jones-outreach

go 1.20

replace github.com/bjornpagen/youtube-apis => ../youtube-apis

replace github.com/bjornpagen/airtable-go => ../airtable-go

require (
	github.com/bjornpagen/airtable-go v0.0.0-20230419122735-83c5b63b90af
	github.com/bjornpagen/prospety-go v0.0.0-20230419124505-35de688fdf3d
	github.com/bjornpagen/youtube-apis v0.0.0-20230419155410-b72f7044b30f
	github.com/davecgh/go-spew v1.1.1
	github.com/sashabaranov/go-openai v1.8.0
	github.com/spf13/cobra v1.7.0
)

require (
	github.com/andres-erbsen/clock v0.0.0-20160526145045-9e14626cd129 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/ratelimit v0.2.0 // indirect
)
