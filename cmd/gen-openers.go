package main

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"

	airtable "github.com/bjornpagen/airtable-go"
	mediadownloader "github.com/bjornpagen/youtube-apis/mediadownloader"
	transcriptor "github.com/bjornpagen/youtube-apis/transcriptor"

	"github.com/spf13/cobra"
)

func runGenOpeners(cmd *cobra.Command, args []string) {
	c, err := New(_prospetyKey, _airtableKey, _openaiKey, _transcriptorKey, _mediadownloaderKey)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.genOpeners(); err != nil {
		log.Fatal(err)
	}
}

func (c *Client) genOpeners() error {
	// fetch all airtable leads
	upstreamLeads, err := c.leadDb.List()
	if err != nil {
		return fmt.Errorf("failed to get airtable leads: %w", err)
	}

	// filter out all leads that already have an opener
	var leadsToGen []airtable.Record[Lead]
	for _, lead := range upstreamLeads {
		if lead.Fields.Opener == "" && lead.Fields.Status == "ready" && lead.Fields.Assignee.Name == "Bjorn Pagen" {
			leadsToGen = append(leadsToGen, lead)
		}
	}

	log.Printf("found %d leads to generate openers for", len(leadsToGen))

	// generate openers for all leads, concurrently
	var wg sync.WaitGroup
	leadsToUpdateSuccesses := make(chan airtable.Record[Lead], len(leadsToGen))
	leadsToUpdateFailures := make(chan airtable.Record[Lead], len(leadsToGen))
	for _, lead := range leadsToGen {
		wg.Add(1)
		go func(lead airtable.Record[Lead]) {
			defer wg.Done()
			successfullyUpdated, err := c.updateSingleOpener(lead.ID, lead.Fields)
			if err != nil {
				log.Printf("failed to update lead %s: %s", lead.ID, err.Error())

				// update the status to failed
				rec := airtable.Record[Lead]{ID: lead.ID, Fields: &Lead{Status: "failed-opener"}}
				leadsToUpdateFailures <- rec

				return
			}
			leadsToUpdateSuccesses <- *successfullyUpdated
		}(lead)
	}

	// wait for all the openers to be generated
	wg.Wait()
	close(leadsToUpdateSuccesses)
	close(leadsToUpdateFailures)

	// convert chan to slice
	var leadsToUpdateSlice []airtable.Record[Lead]
	for lead := range leadsToUpdateSuccesses {
		leadsToUpdateSlice = append(leadsToUpdateSlice, lead)
	}
	log.Printf("%d successful leads", len(leadsToUpdateSlice))

	// convert chan to slice
	var leadsToUpdateFailuresSlice []airtable.Record[Lead]
	for lead := range leadsToUpdateFailures {
		leadsToUpdateFailuresSlice = append(leadsToUpdateFailuresSlice, lead)
	}
	log.Printf("%d failed leads", len(leadsToUpdateFailuresSlice))
	leadsToUpdateSlice = append(leadsToUpdateSlice, leadsToUpdateFailuresSlice...)

	// update the airtable leads
	_, err = c.leadDb.Update(leadsToUpdateSlice)
	if err != nil {
		return fmt.Errorf("failed to update airtable leads: %w", err)
	}

	return nil
}

func (c *Client) updateSingleOpener(recordID string, lead *Lead) (*airtable.Record[Lead], error) {
	// use parseYoutubeChannelId(string(lead.Link))
	// to get the channel id
	channelId, err := parseYoutubeChannelId(string(lead.Link))
	if err != nil {
		return nil, fmt.Errorf("failed to parse channel id: %w", err)
	}

	// get latest video
	video, err := c.getLatestVideo(channelId)
	if err != nil {
		log.Printf("failed to get latest video for %s: %v", channelId, err)
		return nil, fmt.Errorf("failed to get latest video for %s: %w", channelId, err)
	}

	// get transcript
	transcript, err := c.getTranscript(video.ID)
	if err != nil {
		log.Printf("failed to get transcript for video %s: %v", video.ID, err)
		return nil, fmt.Errorf("failed to get transcript for video %s: %w", video.ID, err)
	}

	// get string of whole transcript
	transcriptStr := transcript.String()
	truncVal := 6000

	if len(transcriptStr) == 0 {
		log.Printf("transcript for video %s is empty", video.ID)
		return nil, fmt.Errorf("transcript for video %s is empty", video.ID)
	} else if len(transcriptStr) > truncVal {
		transcriptStr = transcriptStr[:truncVal]
		log.Printf("truncated transcript for video %s to %d chars, lead email: %s", video.ID, truncVal, lead.Email)
	}

	// generate the opener
	log.Printf("generating opener for %s", video.ID)
	opener, err := c.genOpener(transcriptStr)
	if err != nil {
		log.Printf("failed to generate opener for %s: %v", transcriptStr, err)
		return nil, fmt.Errorf("failed to generate opener for %s: %w", transcriptStr, err)
	}

	// update the airtable lead
	lead = &Lead{
		Opener: airtable.ShortText(opener),
		Status: airtable.ShortText("generated-opener"),
	}

	rec := airtable.Record[Lead]{
		ID:     recordID,
		Fields: lead,
	}

	return &rec, nil
}

func (c *Client) getLatestVideo(channelId string) (*mediadownloader.Video, error) {
	// use mediadownloader.GetChannelVideos(channelID string, opts ...getChannelVideosOption) ([]Video, error)
	// get the first video
	// return the video
	videos, err := c.md.GetChannelVideos(channelId)
	if err != nil {
		return nil, err
	}

	if len(videos) == 0 {
		return nil, errors.New("no videos found")
	}

	return &videos[0], nil
}

func (c *Client) getTranscript(videoId string) (*transcriptor.GetTranscriptResponse, error) {
	// use transcriptor.GetTranscript(videoID string, opts ...getTranscriptOption) (*GetTranscriptResponse, error)
	// return the transcript
	transcript, err := c.tr.GetTranscript(videoId)
	if err != nil {
		return nil, err
	}

	return transcript, nil
}

func (c *Client) genOpener(transcript string) (string, error) {
	prompt1 := `Answer the following questions:
1. what is the primary emotion that is evoked by this youtube video?
2. what keeps the audience engaged and interested?
3. what specific personality traits of the youtuber contribute to his/her success?
4. why do you think this youtuber's fans love him?
5. summarize in 3 lines the most entertaining part of this video
6. pretend you're one of his raving fans: write a 1 line response to why you enjoyed his video so much!
answer bullet by bullet, numbered.	
--
%s
	`

	prompt2 := `You are now FirstLineWriterGPT. You are a raving fan of this youtuber, and his content is your favorite on the internet. Write a highly personalized "first line" in an email to the YouTuber. Demonstrate that you have watched the video with specific examples from the video. Come across as human as possible: the job with the first line is to truly demonstrate that I'm not just sending him an email sequence, but a highly personalized and target outreach manually written.
	
You MUST:
1. not include any introduction, such as "hi steven,", as this is already in the email template. i only need the first line, which will be templated into my existing email sequence
2. you cannot, under ANY CIRCUMSTANCES, give a vague or incoherent answer!
3. do not MAKE UP ANECDOTES ABOUT YOURSELF, talk ONLY ABOUT THE CREATOR's VIDEO AND HOW GREAT HE/SHE IS AT CONTENT
4. ONLY WRITE IN FIRST PERSON, ONLY USE PRESENT TENSE

here is some info about the video to help you with your task: i asked ChatGPT these following questions, and here are his responses:
--
%s
--

REMEMBER: Start your response with:
“i loved your latest video! i…”

limit your response to 2 sentences total: cite specific events from the video and tell which was your favorite (to demonstrate you watched it).
`

	// first call
	content := fmt.Sprintf(prompt1, transcript)
	res, err := c.gpt(content)
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	// now do the second call
	content = fmt.Sprintf(prompt2, res)
	res, err = c.gpt(content)
	if err != nil {
		return "", fmt.Errorf("failed to generate opener: %w", err)
	}

	// ai is dumb, force the string to be lowercase
	res = strings.ToLower(res)

	// check if the string is quoted, if so, remove the quotes
	if strings.HasPrefix(res, `"`) && strings.HasSuffix(res, `"`) {
		res = res[1 : len(res)-1]
	}

	// cut off trailing hashtags
	res = strings.Split(res, "#")[0]

	// cut off trailing whitespace
	res = strings.TrimSpace(res)

	return res, nil
}

// parse youtube channel id or handle from the youtube url
// ex: https://www.youtube.com/channel/UCe0TLA0EsQbE-MjuHXevj2A => UCe0TLA0EsQbE-MjuHXevj2A
// ex: https://www.youtube.com/@JohnCena => JohnCena
func parseYoutubeChannelId(inputURL string) (string, error) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}

	if parsedURL.Host != "www.youtube.com" {
		return "", errors.New("invalid URL, not a youtube.com URL")
	}

	channelIDRegexp := regexp.MustCompile(`^/channel/([a-zA-Z0-9_-]+)$`)
	channelHandleRegexp := regexp.MustCompile(`^/@([a-zA-Z0-9_-]+)$`)

	if channelIDMatch := channelIDRegexp.FindStringSubmatch(parsedURL.Path); len(channelIDMatch) == 2 {
		return channelIDMatch[1], nil
	}

	if channelHandleMatch := channelHandleRegexp.FindStringSubmatch(parsedURL.Path); len(channelHandleMatch) == 2 {
		return channelHandleMatch[1], nil
	}

	return "", errors.New("invalid URL, could not find channel ID or handle")
}
