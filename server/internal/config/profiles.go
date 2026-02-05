package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Profile defines a scan profile with scanner options and processing settings.
type Profile struct {
	Profile    ProfileInfo       `toml:"profile"`
	Scanner    ProfileScanner    `toml:"scanner"`
	Processing ProfileProcessing `toml:"processing"`
	Output     ProfileOutput     `toml:"output"`
}

type ProfileInfo struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

type ProfileScanner struct {
	Resolution int     `toml:"resolution"`
	Mode       string  `toml:"mode"`
	Source     string  `toml:"source"`
	PageWidth  float64 `toml:"page_width"`
	PageHeight float64 `toml:"page_height"`
}

type ProfileProcessing struct {
	OptimizeImages   bool           `toml:"optimize_images"`
	Deskew           bool           `toml:"deskew"`
	RemoveBlankPages bool           `toml:"remove_blank_pages"`
	BlankThreshold   float64        `toml:"blank_threshold"`
	OCR              ProfileOCR     `toml:"ocr"`
	Split            ProfileSplit   `toml:"split"`
}

type ProfileOCR struct {
	Enabled  bool   `toml:"enabled"`
	Language string `toml:"language"`
}

type ProfileSplit struct {
	Enabled      bool    `toml:"enabled"`
	ThresholdMM  float64 `toml:"threshold_mm"`
	OverlapMM    float64 `toml:"overlap_mm"`
}

type ProfileOutput struct {
	DefaultTarget string `toml:"default_target"`
}

// ProfileStore manages scan profiles loaded from TOML files.
type ProfileStore struct {
	profiles map[string]*Profile
}

// NewProfileStore creates a new profile store and loads profiles from the given directory.
func NewProfileStore(dir string) (*ProfileStore, error) {
	store := &ProfileStore{
		profiles: make(map[string]*Profile),
	}

	// Add built-in profiles
	store.profiles["standard"] = defaultStandardProfile()
	store.profiles["oversize"] = defaultOversizeProfile()
	store.profiles["photo"] = defaultPhotoProfile()

	// Load from directory if it exists
	if dir != "" {
		if err := store.loadFromDirectory(dir); err != nil {
			return nil, err
		}
	}

	return store, nil
}

// Get returns a profile by name.
func (s *ProfileStore) Get(name string) (*Profile, bool) {
	p, ok := s.profiles[name]
	return p, ok
}

// List returns all available profile names.
func (s *ProfileStore) List() []Profile {
	result := make([]Profile, 0, len(s.profiles))
	for _, p := range s.profiles {
		result = append(result, *p)
	}
	return result
}

// Set adds or updates a profile.
func (s *ProfileStore) Set(name string, p *Profile) {
	s.profiles[name] = p
}

func (s *ProfileStore) loadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read profiles directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read profile %s: %w", entry.Name(), err)
		}

		var profile Profile
		if err := toml.Unmarshal(data, &profile); err != nil {
			return fmt.Errorf("parse profile %s: %w", entry.Name(), err)
		}

		name := strings.TrimSuffix(entry.Name(), ".toml")
		s.profiles[name] = &profile
	}

	return nil
}

func defaultStandardProfile() *Profile {
	return &Profile{
		Profile: ProfileInfo{
			Name:        "Standard Dokument",
			Description: "Farbscan 300 DPI, beidseitig, mit OCR",
		},
		Scanner: ProfileScanner{
			Resolution: 300,
			Mode:       "color",
			Source:     "adf_duplex",
			PageWidth:  210.0,
			PageHeight: 420.0,
		},
		Processing: ProfileProcessing{
			OptimizeImages:   true,
			Deskew:           true,
			RemoveBlankPages: true,
			BlankThreshold:   0.99,
			OCR: ProfileOCR{
				Enabled:  true,
				Language: "deu",
			},
		},
		Output: ProfileOutput{
			DefaultTarget: "paperless",
		},
	}
}

func defaultOversizeProfile() *Profile {
	return &Profile{
		Profile: ProfileInfo{
			Name:        "Überlänge",
			Description: "Für Dokumente länger als A4 (Kontoauszüge, etc.)",
		},
		Scanner: ProfileScanner{
			Resolution: 200,
			Mode:       "gray",
			Source:     "adf_duplex",
			PageWidth:  210.0,
			PageHeight: 0, // Unlimited
		},
		Processing: ProfileProcessing{
			OptimizeImages:   true,
			Deskew:           true,
			RemoveBlankPages: false,
			OCR: ProfileOCR{
				Enabled:  true,
				Language: "deu",
			},
		},
		Output: ProfileOutput{
			DefaultTarget: "paperless",
		},
	}
}

func defaultPhotoProfile() *Profile {
	return &Profile{
		Profile: ProfileInfo{
			Name:        "Foto",
			Description: "Hochauflösender Farbscan für Fotos",
		},
		Scanner: ProfileScanner{
			Resolution: 600,
			Mode:       "color",
			Source:     "flatbed",
			PageWidth:  210.0,
			PageHeight: 297.0,
		},
		Processing: ProfileProcessing{
			OptimizeImages: false,
			Deskew:         false,
			OCR: ProfileOCR{
				Enabled: false,
			},
		},
		Output: ProfileOutput{
			DefaultTarget: "filesystem",
		},
	}
}
