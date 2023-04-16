package main

import (
	"fmt"
	"log"
	"os"

	prospety "github.com/bjornpagen/prospety-go"
	"github.com/davecgh/go-spew/spew"
)

var (
	_prospetyKey string
)

func init() {
	_prospetyKey = os.Getenv("PROSPETY_KEY")

	if _prospetyKey == "" {
		log.Fatal("PROSPETY_KEY is required")
	}
}

func main() {
	s, err := New(_prospetyKey)
	if err != nil {
		log.Fatal(err)
	}
	if err := s.run(); err != nil {
		log.Fatal(err)
	}
}

type Server struct {
	pc *prospety.Client
}

func New(prospetyKey string) (*Server, error) {
	pc, err := prospety.New(prospetyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create prospety client: %w", err)
	}

	return &Server{
		pc: pc,
	}, nil
}

func (s *Server) run() error {
	// Get all the searches
	searches, err := s.pc.GetSearches()
	if err != nil {
		return fmt.Errorf("failed to get projects: %w", err)
	}

	// For each search, get the any (underlying []YoutubeProspect), and coerce to []YoutubeProspect
	var youtubeProspects []prospety.Prospect
	for _, search := range searches {
		prospects, err := s.pc.GetProspects(search.ID)
		if err != nil {
			return fmt.Errorf("failed to get prospects: %w", err)
		}

		youtubeProspects = append(youtubeProspects, prospects...)
	}

	spew.Dump(youtubeProspects)

	return nil
}
