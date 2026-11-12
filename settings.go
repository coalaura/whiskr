package main

import (
	"math"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
)

const DefaultCostLimit = 2.0

type Settings struct {
	mx sync.RWMutex

	timerMx sync.Mutex
	timer   *time.Timer

	Settings map[string]*UserSettings `yaml:"settings"`
}

type UserSettings struct {
	Favorites         []string `yaml:"favorites"`
	CostLimit         *float64 `yaml:"cost_limit,omitempty"`
	RetryMaxRetries   *int64   `yaml:"retry_max_retries,omitempty"`
	RetryInitialDelay *int64   `yaml:"retry_initial_delay,omitempty"`
	RetryMaxDelay     *int64   `yaml:"retry_max_delay,omitempty"`
	TTSModel          *string  `yaml:"tts_model,omitempty"`
	TTSVoice          *string  `yaml:"tts_voice,omitempty"`
	Theme             *string  `yaml:"theme,omitempty"`
	ProviderSorting   *string  `yaml:"provider_sorting,omitempty"`
	ModelBenchmark    *string  `yaml:"model_benchmark,omitempty"`
	ImageResize       *string  `yaml:"image_resize,omitempty"`
	MaxImages         *int64   `yaml:"max_images,omitempty"`
}

func LoadSettings() (*Settings, error) {
	file, err := os.OpenFile(path.Settings, os.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return &Settings{
				Settings: make(map[string]*UserSettings),
			}, nil
		}

		return nil, err
	}

	defer file.Close()

	var st Settings

	err = yaml.NewDecoder(file).Decode(&st)
	if err != nil {
		return nil, err
	}

	return &st, nil
}

func (s *Settings) UnmarshalYAML(data []byte) error {
	return yaml.Unmarshal(data, &s.Settings)
}

func (s *Settings) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(s.Settings)
}

func (s *Settings) Store() error {
	if !s.CancelSchedule() {
		return nil
	}

	s.mx.Lock()
	defer s.mx.Unlock()

	file, err := os.OpenFile(path.Settings, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer file.Close()

	return yaml.NewEncoder(file).Encode(s)
}

func (s *Settings) ScheduleStore() {
	s.timerMx.Lock()
	defer s.timerMx.Unlock()

	if s.timer != nil {
		s.timer.Stop()
	}

	s.timer = time.AfterFunc(10*time.Second, func() {
		s.Store()
	})
}

func (s *Settings) CancelSchedule() bool {
	s.timerMx.Lock()
	defer s.timerMx.Unlock()

	if s.timer == nil {
		return false
	}

	s.timer.Stop()

	s.timer = nil

	return true
}

func (s *Settings) Serialize(username string) map[string]any {
	modelMx.RLock()

	validFavorites := make(map[string]struct{}, len(ModelIDMap))

	for id := range ModelIDMap {
		validFavorites[id] = struct{}{}
	}

	modelMx.RUnlock()

	s.mx.RLock()
	defer s.mx.RUnlock()

	var favorites []string

	user, ok := s.Settings[username]
	if ok && len(user.Favorites) > 0 {
		favorites = make([]string, 0, len(user.Favorites))

		for _, favorite := range user.Favorites {
			if _, ok := validFavorites[favorite]; !ok {
				continue
			}

			favorites = append(favorites, favorite)
		}
	} else {
		favorites = make([]string, 0)
	}

	if user == nil {
		user = &UserSettings{}
	}

	return map[string]any{
		"favorites":           favorites,
		"cost_limit":          userCostLimit(user),
		"retry_max_retries":   user.RetryMaxRetries,
		"retry_initial_delay": user.RetryInitialDelay,
		"retry_max_delay":     user.RetryMaxDelay,
		"tts_model":           user.TTSModel,
		"tts_voice":           user.TTSVoice,
		"theme":               user.Theme,
		"provider_sorting":    user.ProviderSorting,
		"model_benchmark":     user.ModelBenchmark,
		"image_resize":        user.ImageResize,
		"max_images":          user.MaxImages,
	}
}

func (s *Settings) CostLimit(username string) float64 {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return userCostLimit(s.Settings[username])
}

func (s *Settings) MigrateFavoriteModelIDs(models []*Model) bool {
	idsBySlug := make(map[string][]string, len(models))

	for _, model := range models {
		idsBySlug[model.Slug] = append(idsBySlug[model.Slug], model.ID)
	}

	var changed bool

	s.mx.Lock()
	defer s.mx.Unlock()

	for _, user := range s.Settings {
		favorites := make([]string, 0, len(user.Favorites))
		seen := make(map[string]struct{}, len(user.Favorites))

		for _, favorite := range user.Favorites {
			ids := []string{favorite}

			if !IsModelShortID(favorite) {
				migrated := idsBySlug[favorite]
				if len(migrated) > 0 {
					ids = migrated
				}
			}

			for _, id := range ids {
				if _, ok := seen[id]; ok {
					continue
				}

				seen[id] = struct{}{}
				favorites = append(favorites, id)
			}
		}

		if !slices.Equal(user.Favorites, favorites) {
			user.Favorites = favorites
			changed = true
		}
	}

	return changed
}

func (s *Settings) SetFavorites(username string, favorites []string) {
	s.mx.Lock()
	defer s.mx.Unlock()

	user := s.getLocked(username)

	user.Favorites = favorites

	s.ScheduleStore()
}

func (s *Settings) getLocked(username string) *UserSettings {
	user, ok := s.Settings[username]
	if !ok {
		user = &UserSettings{}

		s.Settings[username] = user
	}

	return user
}

func validCostLimit(limit float64) bool {
	return limit > 0 && !math.IsNaN(limit) && !math.IsInf(limit, 0)
}

func userCostLimit(user *UserSettings) float64 {
	if user != nil && user.CostLimit != nil && validCostLimit(*user.CostLimit) {
		return *user.CostLimit
	}

	return DefaultCostLimit
}
