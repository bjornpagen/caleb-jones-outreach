package main

import (
	"fmt"
	"log"
	"os"

	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"

	airtable "github.com/bjornpagen/airtable-go"
	prospety "github.com/bjornpagen/prospety-go"
	mediadownloader "github.com/bjornpagen/youtube-apis/mediadownloader"
	transcriptor "github.com/bjornpagen/youtube-apis/transcriptor"
)

var (
	_prospetyKey        string
	_airtableKey        string
	_openaiKey          string
	_transcriptorKey    string
	_mediadownloaderKey string
)

func init() {
	_prospetyKey = os.Getenv("PROSPETY_KEY")
	_airtableKey = os.Getenv("AIRTABLE_KEY")
	_openaiKey = os.Getenv("OPENAI_KEY")
	_transcriptorKey = os.Getenv("TRANSCRIPTOR_KEY")
	_mediadownloaderKey = os.Getenv("MEDIADOWNLOADER_KEY")

	if _prospetyKey == "" {
		log.Fatal("PROSPETY_KEY is required")
	}
	if _airtableKey == "" {
		log.Fatal("AIRTABLE_KEY is required")
	}
	if _openaiKey == "" {
		log.Fatal("OPENAI_KEY is required")
	}
	if _transcriptorKey == "" {
		log.Fatal("TRANSCRIPTOR_KEY is required")
	}
	if _mediadownloaderKey == "" {
		log.Fatal("MEDIADOWNLOADER_KEY is required")
	}

	// Add subcommands
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(genOpeners)
}

var (
	rootCmd = &cobra.Command{
		Use:   "main",
		Short: "A CLI tool to manage leads and activities",
	}

	mergeCmd = &cobra.Command{
		Use:   "merge",
		Short: "Upload new Prospety Prospects to Airtable Leads",
		Run:   runMerge,
	}

	genOpeners = &cobra.Command{
		Use:   "gen-openers",
		Short: "Generate openers for all leads that don't have one",
		Run:   runGenOpeners,
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

type Client struct {
	pc *prospety.Client
	db *airtable.Client
	oc *openai.Client
	tr *transcriptor.Client
	md *mediadownloader.Client

	leadDb     *airtable.Table[Lead]
	activityDb *airtable.Table[Activity]
}

func New(prospetyKey, airtableKey, openaiKey, transcriptorKey, mediadownloaderKey string) (*Client, error) {
	pc, err := prospety.New(prospetyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create prospety client: %w", err)
	}

	db, err := airtable.New(airtableKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create airtable client: %w", err)
	}

	tr, err := transcriptor.New(transcriptorKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcriptor client: %w", err)
	}

	md, err := mediadownloader.New(mediadownloaderKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create mediadownloader client: %w", err)
	}

	c := &Client{
		pc: pc,
		db: db,
		oc: openai.NewClient(openaiKey),
		tr: tr,
		md: md,
	}

	c.leadDb = NewLeadDB(c.db)
	c.activityDb = NewActivityDB(c.db)

	return c, nil
}
