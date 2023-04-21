package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	airtable "github.com/bjornpagen/airtable-go"
	"github.com/spf13/cobra"
)

func runGenName(cmd *cobra.Command, args []string) {
	c, err := New(_prospetyKey, _airtableKey, _openaiKey, _transcriptorKey, _mediadownloaderKey)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.genName(); err != nil {
		log.Fatal(err)
	}
}

func (c *Client) genName() error {
	// fetch all airtable leads
	upstreamLeads, err := c.leadDb.List()
	if err != nil {
		return fmt.Errorf("failed to get airtable leads: %w", err)
	}

	// filter out all leads that already have an name
	var leadsToGen []airtable.Record[Lead]
	for _, lead := range upstreamLeads {
		if lead.Fields.Status == "ready-name" && lead.Fields.Assignee.Name == "Bjorn Pagen" {
			leadsToGen = append(leadsToGen, lead)
		}
	}

	log.Printf("found %d leads to generate names for", len(leadsToGen))

	// generate names for all leads, concurrently
	var wg sync.WaitGroup
	leadsToUpdateSuccesses := make(chan airtable.Record[Lead], len(leadsToGen))
	leadsToUpdateFailures := make(chan airtable.Record[Lead], len(leadsToGen))
	for _, lead := range leadsToGen {
		wg.Add(1)
		go func(lead airtable.Record[Lead]) {
			defer wg.Done()
			successfullyUpdated, err := c.updateSingleName(lead.ID, lead.Fields)
			if err != nil {
				log.Printf("failed to update lead %s: %s", lead.ID, err.Error())

				// update the status to failed
				rec := airtable.Record[Lead]{ID: lead.ID, Fields: &Lead{Status: "failed-name"}}
				leadsToUpdateFailures <- rec

				return
			}
			leadsToUpdateSuccesses <- *successfullyUpdated
		}(lead)
	}

	wg.Wait()
	close(leadsToUpdateSuccesses)
	close(leadsToUpdateFailures)

	// convert to slice
	var leadsToUpdateSlice []airtable.Record[Lead]
	for lead := range leadsToUpdateSuccesses {
		leadsToUpdateSlice = append(leadsToUpdateSlice, lead)
	}

	// update all the leads that were successfully updated
	if _, err := c.leadDb.Update(leadsToUpdateSlice); err != nil {
		log.Printf("failed to update leads: %s", err.Error())
	}
	log.Printf("%d successful leads", len(leadsToUpdateSlice))

	// convert to slice
	var leadsToUpdateSliceFailures []airtable.Record[Lead]
	for lead := range leadsToUpdateFailures {
		leadsToUpdateSliceFailures = append(leadsToUpdateSliceFailures, lead)
	}

	// update all the leads that were successfully updated
	if _, err := c.leadDb.Update(leadsToUpdateSliceFailures); err != nil {
		log.Printf("failed to update leads: %s", err.Error())
	}
	log.Printf("%d failed leads", len(leadsToUpdateSliceFailures))

	return nil
}

func (c *Client) updateSingleName(id string, lead *Lead) (*airtable.Record[Lead], error) {
	// retrieve the base64 encoded gob from the lead
	encodedGob := lead.Gob

	// decode the base64 encoded gob
	prospect, err := decodeStringGob(string(encodedGob))
	if err != nil {
		return nil, fmt.Errorf("failed to decode gob: %w", err)
	}

	// generate the json GPT payload
	type payload struct {
		YouTubeName     string `json:"youtube_name"`
		YouTubeKeywords string `json:"youtube_keywords"`
		YouTubeEmail    string `json:"youtube_email"`
	}

	// build our payload from the prospect
	p := payload{
		YouTubeName:     prospect.Name,
		YouTubeKeywords: strings.Join(prospect.Keywords, ","),
		YouTubeEmail:    prospect.Email,
	}

	// convert to json, send to gpt
	jsonPayload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	prompt := `here is the raw json data scraped from a youtube channel. the issue with this data is that the youtube_name may not accurately represent the true name of the youtuber.
using your inference skills with the youtube_name and youtube_email, please infer the true name of the youtuber.
also, please infer the main niche of the youtuber.
if the youtuber operates in a language primarily in a non-english language, please set the detected_foreign_youtube_channel to true. otherwise, leave it as false.
your returned json object should be in the following schema:
-- input --
{
	"youtube_name": "NowHereBlow",
	"youtube_keywords": "travel documentary,travel around the world,dubai mall,dubai city,tour the world,world tourism,lonely tour,abu dhabi,india,Dubai,travel vlog,world travel,kerala,nowhereblow,nhb",
	"youtube_email": "connect@nowhereblow.com"
}
-- output --
{
	"inferred_name": "NowHereBlow",
	"inferred_main_niche": "travel",
	"detected_foreign_youtube_channel": false
}
-- input --
{
"youtube_name": "BK Crypto Trader - The Boss of Bitcoin",
"youtube_keywords": "boss of bitcoin,crypto boss,bk bitcoin,bk crypto,btc usd,btc news,btc today,bitcoin price analysis,btc price,free btc,bitcoin price prediction,bitcoin prediction,free bitcoin,bitcoin news,btc,bitcoin price,bk,bitcoin analysis,btc analysis,free crypto,bitcoin today,bitcoin 2021,bitcoin news today,btc live,btc ta,crypto news,bitcoin live,crypto trading,bitcoin price today,crypto,btcusd,usdt,BK Crypto Trader,xrp ripple,bitcoin trading",
"youtube_email": "bkbitcoin01@gmail.com"
}
-- output --
{
"inferred_name": "BK Crypto Trader",
"inferred_main_niche": "crypto",
"detected_foreign_youtube_channel": false
}
-- input --
{
"youtube_name": "Aaron Luján",
"youtube_keywords": "bitcoin,xrp,btc,trading en vivo,analisis bitcoin,eth,bitcoin analisis,litecoin,scalping en vivo,analisis btc,bitcoin bajista,criptomoedas",
"youtube_email": "aaron@paguertrading.com"
}
-- output --
{
"inferred_name": "Aaron Luján",
"inferred_main_niche": "bitcoin",
"detected_foreign_youtube_channel": true
}
-- input --
{
"youtube_name": "potatofish yu",
"youtube_keywords": "旅遊,吃喝玩樂,分享,youtube,travel,tasmania,塔斯曼尼亞,情侶,購物,娛樂,vlog,移民生活,澳洲移民,澳洲生活,澳洲自由行,vlogger,haul,beauty,塔斯馬尼亞,lifestyle,懷孕,懷孕vlog,湊B,育兒,生產,孕婦,懷孕準備,我的湊B生活,新手爸媽,新手父母",
"youtube_email": "potatofishyu@gmail.com"
}
-- output --
{
"inferred_name": "Potatofish Yu",
"inferred_main_niche": "旅遊",
"detected_foreign_youtube_channel": true
}
-- input --
{
	"youtube_name": "JustJordan33",
	"youtube_keywords": "justjordan33,jordan,just jordan 33,sister,fun,cute,girl,teen,challenge,challenges,challenge videos,family,nice,friendly,happy,teen,hawaii,teenager,lds,travel,travel vlogs,jordan williams",
	"youtube_email": "justjordan33@gmail.com"
}
-- output --
{
	"inferred_name": "Jordan Williams",
	"inferred_main_niche": "travel",
	"detected_foreign_youtube_channel": false
}
-- input --
{
	"youtube_name": "Elliott Hulse’s STRENGTH CAMP",
	"youtube_keywords": "Elliott Hulse,Strength Training,Weightlifting,Strongman,Powerlifting,Bodybuilding",
	"youtube_email": "colleen@elliotthulse.com"
}
-- output --
{
	"inferred_name": "Elliott Hulse",
	"inferred_main_niche": "bodybuilding",
	"detected_foreign_youtube_channel": false
}
--
here is your input. please respond with only the json object. do not include any other characters.
--
%s`

	gptPayloadStr := fmt.Sprintf(prompt, string(jsonPayload))
	// use gpt
	gptResponse, err := c.gpt(gptPayloadStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get gpt response: %w", err)
	}

	// attempt to unmarshal into return payload
	type returnPayload struct {
		InferredName                  string `json:"inferred_name"`
		InferredMainNiche             string `json:"inferred_main_niche"`
		DetectedForeignYouTubeChannel bool   `json:"detected_foreign_youtube_channel"`
	}

	returnPayloadObj := &returnPayload{}
	if err := json.Unmarshal([]byte(gptResponse), returnPayloadObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gpt response: %w", err)
	}

	ret := &airtable.Record[Lead]{
		ID: id,
		Fields: &Lead{
			InferredName:  airtable.ShortText(returnPayloadObj.InferredName),
			InferredNiche: airtable.ShortText(strings.ToLower(returnPayloadObj.InferredMainNiche)),
		},
	}

	// check if foreign bool is set, then set status to "failed-foreign"
	if returnPayloadObj.DetectedForeignYouTubeChannel {
		ret.Fields.Status = airtable.ShortText("failed-foreign")
	} else {
		// otherwise, set status to "success-name"
		ret.Fields.Status = airtable.ShortText("success-name")
	}

	return ret, nil
}
