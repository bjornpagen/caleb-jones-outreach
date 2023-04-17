package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/davecgh/go-spew/spew"
	openai "github.com/sashabaranov/go-openai"

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
}

func main() {
	s, err := New(_prospetyKey, _airtableKey, _openaiKey)
	if err != nil {
		log.Fatal(err)
	}
	if err := s.run(); err != nil {
		log.Fatal(err)
	}
}

type Server struct {
	pc *prospety.Client
	db *airtable.Client
	oc *openai.Client

	leadDb     *airtable.Table[Lead]
	activityDb *airtable.Table[Activity]
}

func New(prospetyKey, airtableKey, openaiKey string) (*Server, error) {
	pc, err := prospety.New(prospetyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create prospety client: %w", err)
	}

	db, err := airtable.New(airtableKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create airtable client: %w", err)
	}

	return &Server{
		pc: pc,
		db: db,
		oc: openai.NewClient(openaiKey),
	}, nil
}

func (s *Server) run() error {
	s.leadDb = NewLeadDB(s.db)
	s.activityDb = NewActivityDB(s.db)

	// // get all leads
	// leads, err := s.leadDb.List()
	// if err != nil {
	// 	return fmt.Errorf("failed to get leads: %w", err)
	// }

	// // Unwrap all the Leads
	// var airtableLeads []Lead
	// for _, lead := range leads {
	// 	airtableLeads = append(airtableLeads, *lead.Fields)
	// }

	// // Marshall all the Leads to JSON
	// var strings []string
	// for _, lead := range airtableLeads {
	// 	str, err := json.Marshal(lead)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to marshal lead: %w", err)
	// 	}
	// 	strings = append(strings, string(str))
	// }

	// // Print all the Leads
	// spew.Dump(strings)

	// res, err := s.gpt("Hello")
	// if err != nil {
	// 	return fmt.Errorf("failed to get gpt: %w", err)
	// }

	// fmt.Println(res)

	res, err := dumpType[Activity]()
	if err != nil {
		return fmt.Errorf("failed to dump type: %w", err)
	}

	prompt := fmt.Sprintf("Generate CREATIVE sample data for the following data structure. Respond only in JSON, with no additional text. Response must be valid JSON because it is fed directly into an Unmarshall function. \n\n%s\n\n", res)

	res, err = s.gpt(prompt)
	if err != nil {
		return fmt.Errorf("failed to get gpt: %w", err)
	}

	fmt.Println(res)

	return nil
}

func (s *Server) getProspects() ([]prospety.Prospect, error) {
	// Get all the searches
	searches, err := s.pc.GetSearches()
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}

	// For each search, get the any (underlying []YoutubeProspect), and coerce to []YoutubeProspect
	var youtubeProspects []prospety.Prospect
	for _, search := range searches {
		prospects, err := s.pc.GetProspects(search.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get prospects: %w", err)
		}

		youtubeProspects = append(youtubeProspects, prospects...)
	}

	return youtubeProspects, nil
}

// Airtable Types

type Lead struct {
	Assignee   airtable.User         `json:"Assignee"`
	Topic      airtable.SingleSelect `json:"Topic"`
	Status     airtable.ShortText    `json:"Status"`
	Name       airtable.ShortText    `json:"Name"`
	FollowersK airtable.Number       `json:"Followers (K)"`
	Platform   airtable.SingleSelect `json:"Platform"`
	Link       airtable.URL          `json:"Link"`
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

// AI stuff
func (s *Server) gpt(prompt string) (response string, err error) {
	res, err := s.oc.CreateChatCompletion(
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

func dumpType[T any]() (string, error) {
	empty := new(T)
	str, err := json.Marshal(empty)
	if err != nil {
		return "", fmt.Errorf("failed to marshal lead: %w", err)
	}

	// create buffer to hold the result of spew
	var buf bytes.Buffer
	spew.Fdump(&buf, empty, string(str))

	return buf.String(), nil
}
