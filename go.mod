module github.com/bjornpagen/caleb-jones-outreach

go 1.20

replace github.com/bjornpagen/airtable-go => ../airtable-go

require (
	github.com/bjornpagen/airtable-go v0.0.0-20230417112227-a52bd310d018
	github.com/bjornpagen/prospety-go v0.0.0-20230416225100-37324f0c457a
	github.com/davecgh/go-spew v1.1.1
	github.com/sashabaranov/go-openai v1.8.0
)

require (
	github.com/andres-erbsen/clock v0.0.0-20160526145045-9e14626cd129 // indirect
	go.uber.org/ratelimit v0.2.0 // indirect
)
