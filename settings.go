package main

import (
	"os"
	"slices"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
)

type Settings struct {
	mx sync.RWMutex

	timerMx sync.Mutex
	timer   *time.Timer

	Settings map[string]*UserSettings `yaml:"settings"`
}

type UserSettings struct {
	Favorites []string `yaml:"favorites"`
}

func LoadSettings() (*Settings, error) {
	file, err := os.OpenFile("settings.yml", os.O_RDONLY, 0)
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

	file, err := os.OpenFile("settings.yml", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
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

	return map[string]any{
		"favorites": favorites,
	}
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
				if migrated := idsBySlug[favorite]; len(migrated) > 0 {
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
