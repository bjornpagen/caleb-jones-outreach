package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"

	airtable "github.com/bjornpagen/airtable-go"
	prospety "github.com/bjornpagen/prospety-go"
)

var (
	_prospetyKey string
	_airtableKey string
	_openaiKey   string
)

func init() {
	_prospetyKey = os.Getenv("PROSPETY_KEY")
	_airtableKey = os.Getenv("AIRTABLE_KEY")
	_openaiKey = os.Getenv("OPENAI_KEY")

	if _prospetyKey == "" {
		log.Fatal("PROSPETY_KEY is required")
	}
	if _airtableKey == "" {
		log.Fatal("AIRTABLE_KEY is required")
	}
	if _openaiKey == "" {
		log.Fatal("OPENAI_KEY is required")
	}

	// Add subcommands
	rootCmd.AddCommand(mergeCmd)
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
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runMerge(cmd *cobra.Command, args []string) {
	c, err := New(_prospetyKey, _airtableKey, _openaiKey)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.mergeProspetyLeads(); err != nil {
		log.Fatal(err)
	}
}

type Client struct {
	pc *prospety.Client
	db *airtable.Client
	oc *openai.Client

	leadDb     *airtable.Table[Lead]
	activityDb *airtable.Table[Activity]
}

func New(prospetyKey, airtableKey, openaiKey string) (*Client, error) {
	pc, err := prospety.New(prospetyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create prospety client: %w", err)
	}

	db, err := airtable.New(airtableKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create airtable client: %w", err)
	}

	c := &Client{
		pc: pc,
		db: db,
		oc: openai.NewClient(openaiKey),
	}

	c.leadDb = NewLeadDB(c.db)
	c.activityDb = NewActivityDB(c.db)

	return c, nil
}

func (c *Client) getProspects() ([]prospety.Prospect, error) {
	// Get all the searches
	searches, err := c.pc.GetSearches()
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}

	// For each search, get the any (underlying []YoutubeProspect), and coerce to []YoutubeProspect
	var youtubeProspects []prospety.Prospect
	for _, search := range searches {
		prospects, err := c.pc.GetProspects(search.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get prospects: %w", err)
		}

		youtubeProspects = append(youtubeProspects, prospects...)
	}

	return youtubeProspects, nil
}

func (c *Client) getAirtableLeads() ([]Lead, error) {
	// get all leads
	leads, err := c.leadDb.List()
	if err != nil {
		return nil, fmt.Errorf("failed to get leads: %w", err)
	}

	// Unwrap all the Leads
	var airtableLeads []Lead
	for _, lead := range leads {
		airtableLeads = append(airtableLeads, *lead.Fields)
	}

	return airtableLeads, nil
}

func (c *Client) mergeProspetyLeads() error {
	// Get all the prospects
	prospects, err := c.getProspects()
	if err != nil {
		return fmt.Errorf("failed to get prospects: %w", err)
	}

	// convert all to leadDetails
	var leads []leadDetails
	for _, prospect := range prospects {
		leads = append(leads, *prospectToLeadDetails(prospect))
	}

	// convert all to Lead
	var airtableLeads []Lead
	for _, lead := range leads {
		airtableLeads = append(airtableLeads, Lead{
			leadDetails: lead,
			salesDetails: salesDetails{
				Assignee: airtable.User{
					Id: "usrVUTmD0O5A2eaEW",
				},
				Status: airtable.ShortText(""),
			},
		})
	}

	// fetch all airtable leads
	upstreamLeads, err := c.getAirtableLeads()
	if err != nil {
		return fmt.Errorf("failed to get airtable leads: %w", err)
	}

	// create an map[airtable.Email]Lead from airtable leads
	upstreamLeadsMap := make(map[airtable.Email]Lead)
	for _, lead := range upstreamLeads {
		upstreamLeadsMap[lead.Email] = lead
	}

	// for each lead in airtableLeads, check if it exists in upstreamLeadsMap
	// create a new slice of airtableLeads that only contains the ones that don't exist
	var newLeads []Lead
	for _, lead := range airtableLeads {
		if _, ok := upstreamLeadsMap[lead.Email]; !ok {
			newLeads = append(newLeads, lead)
		}
	}

	// create it
	res, err := c.leadDb.Create(newLeads)
	if err != nil {
		return fmt.Errorf("failed to create lead: %w", err)
	}

	log.Printf("Created %d new leads", len(res))

	return nil
}

// Airtable Types

type Lead struct {
	salesDetails
	leadDetails
}

type leadDetails struct {
	Topic      airtable.SingleSelect `json:"Topic,omitempty"`
	Name       airtable.ShortText    `json:"Name,omitempty"`
	FollowersK airtable.Number       `json:"Followers (K),omitempty"`
	Platform   airtable.SingleSelect `json:"Platform,omitempty"`
	Link       airtable.URL          `json:"Link,omitempty"`
	Email      airtable.Email        `json:"Email,omitempty"`
	Phone      airtable.Phone        `json:"Phone,omitempty"`
}

type salesDetails struct {
	Assignee airtable.User      `json:"Assignee,omitempty"`
	Status   airtable.ShortText `json:"Status,omitempty"`
}

type Activity struct {
	NewLeadsInPipeline       airtable.Number    `json:"New Leads in Pipeline"`
	ContactsSinceLastUpdate  airtable.Number    `json:"Contacts since Last Update"`
	ResponsesSinceLastUpdate airtable.Number    `json:"Responses since Last Update"`
	Update                   airtable.Number    `json:"Update"`
	Salesperson              airtable.User      `json:"Salesperson"`
	IncrementalResponseRate  airtable.Number    `json:"Incremental Response Rate"`
	Created                  airtable.ShortText `json:"Created"`
}

func NewLeadDB(c *airtable.Client) *airtable.Table[Lead] {
	return airtable.NewTable[Lead](c, "appl2x7vwQfJClY42", "tblQcKRYGoq7kIxVN")
}

func NewActivityDB(c *airtable.Client) *airtable.Table[Activity] {
	return airtable.NewTable[Activity](c, "appl2x7vwQfJClY42", "tblfPpzBCMhjXRCJg")
}

// Utils

func prospectToLeadDetails(prospect prospety.Prospect) *leadDetails {
	return &leadDetails{
		//Topic:      airtable.SingleSelect(capitalizeFirst(prospect.Keywords[0])),
		Name:       airtable.ShortText(prospect.Name),
		FollowersK: airtable.Number(prospect.Subscribers / 1000),
		Platform:   airtable.SingleSelect("YouTube"),
		Link:       airtable.URL(prospect.URL),
		Email:      airtable.Email(prospect.Email),
		Phone:      airtable.Phone(prospect.Phone),
	}
}

// AI stuff

func dump[T any](in T) (string, error) {
	str, err := json.Marshal(&in)
	if err != nil {
		return "", fmt.Errorf("failed to marshal lead: %w", err)
	}

	// create buffer to hold the result of spew
	var buf bytes.Buffer
	spew.Fdump(&buf, &in, string(str))

	return buf.String(), nil
}

func capitalizeFirst(s string) string {
	return strings.ToUpper(s[:1]) + s[1:]
}

func (c *Client) gpt(prompt string) (response string, err error) {
	res, err := c.oc.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	response = res.Choices[0].Message.Content
	return response, nil
}
