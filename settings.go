package main

import (
	"os"
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
	s.mx.RLock()
	defer s.mx.RUnlock()

	user, ok := s.Settings[username]
	if !ok {
		user = &UserSettings{
			Favorites: make([]string, 0),
		}
	}

	return map[string]any{
		"favorites": user.Favorites,
	}
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
