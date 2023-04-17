package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	prospety "github.com/bjornpagen/prospety-go"
	"github.com/davecgh/go-spew/spew"

	"github.com/bjornpagen/caleb-jones-outreach/airtable"
)

var (
	_prospetyKey string
	_airtableKey string
)

func init() {
	_prospetyKey = os.Getenv("PROSPETY_KEY")
	_airtableKey = os.Getenv("AIRTABLE_KEY")

	if _prospetyKey == "" {
		log.Fatal("PROSPETY_KEY is required")
	}
	if _airtableKey == "" {
		log.Fatal("AIRTABLE_KEY is required")
	}
}

func main() {
	s, err := New(_prospetyKey, _airtableKey)
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
}

func New(prospetyKey, airtableKey string) (*Server, error) {
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
	}, nil
}

func (s *Server) run() error {
	leadDb := NewLeadDB(s.db)

	// get all leads
	leads, err := leadDb.List()
	if err != nil {
		return fmt.Errorf("failed to get leads: %w", err)
	}

	// Unwrap all the Leads
	var airtableLeads []Lead
	for _, lead := range leads {
		airtableLeads = append(airtableLeads, *lead.Fields)
	}

	// Marshall all the Leads to JSON
	var strings []string
	for _, lead := range airtableLeads {
		str, err := json.Marshal(lead)
		if err != nil {
			return fmt.Errorf("failed to marshal lead: %w", err)
		}
		strings = append(strings, string(str))
	}

	// Print all the Leads
	spew.Dump(strings)

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
