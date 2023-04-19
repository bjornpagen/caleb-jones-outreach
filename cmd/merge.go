package main

import (
	"fmt"
	"log"

	airtable "github.com/bjornpagen/airtable-go"
	"github.com/spf13/cobra"
)

func runMerge(cmd *cobra.Command, args []string) {
	c, err := New(_prospetyKey, _airtableKey, _openaiKey, _transcriptorKey, _mediadownloaderKey)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.mergeProspetyLeads(); err != nil {
		log.Fatal(err)
	}
}

func (c *Client) mergeProspetyLeads() error {
	// Get all the prospects
	prospects, err := c.getProspects()
	if err != nil {
		return fmt.Errorf("failed to merge: %w", err)
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
				Status: airtable.ShortText(""),
			},
		})
	}

	// filter through all leads and remove any leads with duplicate email addresses
	// create a map[airtable.Email]Lead
	airtableLeadsMap := make(map[airtable.Email]Lead)
	for _, lead := range airtableLeads {
		airtableLeadsMap[lead.Email] = lead
	}

	// create a new slice of airtableLeads that only contains the unique ones
	var uniqueLeads []Lead
	for _, lead := range airtableLeadsMap {
		uniqueLeads = append(uniqueLeads, lead)
	}

	// set airtableLeads to uniqueLeads
	airtableLeads = uniqueLeads

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
