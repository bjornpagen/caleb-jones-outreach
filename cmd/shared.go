package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	airtable "github.com/bjornpagen/airtable-go"
	prospety "github.com/bjornpagen/prospety-go"
	"github.com/davecgh/go-spew/spew"
	openai "github.com/sashabaranov/go-openai"
)

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

// Airtable Types

type Lead struct {
	Topic         airtable.SingleSelect `json:"Topic,omitempty"`
	Name          airtable.ShortText    `json:"Name,omitempty"`
	FollowersK    airtable.Number       `json:"Followers (K),omitempty"`
	Platform      airtable.SingleSelect `json:"Platform,omitempty"`
	Link          airtable.URL          `json:"Link,omitempty"`
	Email         airtable.Email        `json:"Email,omitempty"`
	Phone         airtable.Phone        `json:"Phone,omitempty"`
	Gob           airtable.ShortText    `json:"Gob,omitempty"`
	Opener        airtable.ShortText    `json:"Opener,omitempty"`
	Assignee      *airtable.User        `json:"Assignee,omitempty"`
	Status        airtable.ShortText    `json:"Status,omitempty"`
	InferredName  airtable.ShortText    `json:"Inferred Name,omitempty"`
	InferredNiche airtable.ShortText    `json:"Inferred Niche,omitempty"`
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

func prospectToLeadDetails(prospect prospety.Prospect) *Lead {
	// turn the whole prospect into a base64 encoded gob
	gobStr, err := encodeStringGob(&prospect)
	if err != nil {
		log.Fatalf("failed to encode prospect: %v", err)
	}

	return &Lead{
		Topic:      airtable.SingleSelect(capitalizeFirst(prospect.Keywords[0])),
		Name:       airtable.ShortText(prospect.Name),
		FollowersK: airtable.Number(prospect.Subscribers / 1000),
		Platform:   airtable.SingleSelect("YouTube"),
		Link:       airtable.URL(prospect.URL),
		Email:      airtable.Email(prospect.Email),
		Phone:      airtable.Phone(prospect.Phone),
		Gob:        airtable.ShortText(gobStr),
	}
}

func encodeStringGob(p *prospety.Prospect) (string, error) {
	// turn the whole prospect into a base64 encoded gob
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(p); err != nil {
		log.Fatalf("failed to encode prospect: %v", err)
	}

	// now turn it into a string
	gobStr := base64.StdEncoding.EncodeToString(buf.Bytes())

	return gobStr, nil
}

func decodeStringGob(s string) (p *prospety.Prospect, err error) {
	// decode the string into a gob
	gobBytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// initialize the prospect
	p = &prospety.Prospect{}

	// decode the gob into the prospect
	dec := gob.NewDecoder(bytes.NewReader(gobBytes))
	if err := dec.Decode(p); err != nil {
		return nil, fmt.Errorf("failed to decode gob: %w", err)
	}

	return p, nil
}

// AI stuff
func capitalizeFirst(s string) string {
	return strings.ToUpper(s[:1]) + s[1:]
}

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

func (c *Client) gpt(prompt string) (response string, err error) {
	c.gptLimiter.Take()
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
