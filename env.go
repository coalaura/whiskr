package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"golang.org/x/crypto/bcrypt"
)

type EnvTokens struct {
	Secret     string `yaml:"secret"`
	OpenRouter string `yaml:"openrouter"`
	Exa        string `yaml:"exa"`
	GitHub     string `yaml:"github"`
}

type EnvServer struct {
	Port int64 `yaml:"port"`
}

type EnvSettings struct {
	CleanContent    bool  `yaml:"cleanup"`
	Timeout         int64 `yaml:"timeout"`
	RefreshInterval int64 `yaml:"refresh-interval"`
}

type EnvModels struct {
	TitleModel      string `yaml:"title-model"`
	ImageGeneration bool   `yaml:"image-generation"`
	Transformation  string `yaml:"transformation"`
	Filters         string `yaml:"filters"`

	filters *Filters
}

type EnvUI struct {
	ReducedMotion bool `yaml:"reduced-motion"`
}

type EnvUser struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type EnvAuthentication struct {
	lookup map[string]*EnvUser

	Enabled bool       `yaml:"enabled"`
	Users   []*EnvUser `yaml:"users"`
}

type Environment struct {
	dmx sync.RWMutex // data mutex
	fmx sync.Mutex   // file mutex

	Debug          bool              `yaml:"debug"`
	Tokens         EnvTokens         `yaml:"tokens"`
	Server         EnvServer         `yaml:"server"`
	Settings       EnvSettings       `yaml:"settings"`
	Models         EnvModels         `yaml:"models"`
	UI             EnvUI             `yaml:"ui"`
	Authentication EnvAuthentication `yaml:"authentication"`
}

func LoadEnv() (*Environment, error) {
	// defaults
	cfg := &Environment{
		Server: EnvServer{
			Port: 3443,
		},
		Settings: EnvSettings{
			CleanContent:    true,
			Timeout:         1200,
			RefreshInterval: 30,
		},
		Models: EnvModels{
			ImageGeneration: true,
		},
	}

	file, err := os.OpenFile("config.yml", os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	err = yaml.NewDecoder(file).Decode(cfg)
	if err != nil {
		return nil, err
	}

	err = cfg.Init()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (e *Environment) Addr() string {
	return fmt.Sprintf(":%d", e.Server.Port)
}

func (e *Environment) Init() error {
	var store bool

	// print if debug is enabled
	if e.Debug {
		log.Warnln("Debug mode enabled")
	}

	// print if image generation is enabled
	if e.Models.ImageGeneration {
		log.Warnln("Image generation enabled")
	} else {
		log.Warnln("Image generation disabled")
	}

	// check if server secret is set
	if e.Tokens.Secret == "" {
		log.Warnln("Missing tokens.secret, generating new")

		secret, err := CreateSecret(32)
		if err != nil {
			return err
		}

		e.Tokens.Secret = secret

		store = true
	}

	// check if openrouter token is set
	if e.Tokens.OpenRouter == "" {
		return errors.New("missing tokens.openrouter")
	}

	// check if exa token is set
	if e.Tokens.Exa == "" {
		log.Warnln("Missing token.exa, web search unavailable")
	}

	// check if github token is set
	if e.Tokens.GitHub == "" {
		log.Warnln("Missing token.github, limited api requests")
	}

	// check if port is valid
	if e.Server.Port <= 0 || e.Server.Port >= 65535 {
		return fmt.Errorf("invalid port %d", e.Server.Port)
	}

	// default title model
	if e.Models.TitleModel == "" {
		e.Models.TitleModel = "google/gemini-2.5-flash-lite"
	}

	// default transformation method
	if e.Models.Transformation == "" {
		e.Models.Transformation = "middle-out"
	}

	filters, err := ParseFilters(e.Models.Filters)
	if err != nil {
		return err
	}

	e.Models.filters = filters

	// default timeout
	if e.Settings.Timeout <= 0 {
		e.Settings.Timeout = 300
	}

	// default model refresh interval
	if e.Settings.RefreshInterval <= 0 {
		e.Settings.RefreshInterval = 30
	}

	// create user lookup map
	e.Authentication.lookup = make(map[string]*EnvUser)

	for _, user := range e.Authentication.Users {
		if strings.HasPrefix(user.Password, "text=") {
			log.Warnf("User %q has plaintext password, generating hash\n", user.Username)

			hash, err := bcrypt.GenerateFromPassword([]byte(user.Password[5:]), bcrypt.DefaultCost)
			if err != nil {
				return err
			}

			user.Password = string(hash)

			store = true
		}

		e.Authentication.lookup[user.Username] = user
	}

	if store {
		if err := e.Store(); err != nil {
			return err
		}

		log.Println("Updated config.yml")
	}

	return nil
}

func (e *Environment) Store() error {
	var (
		buffer   bytes.Buffer
		comments = yaml.CommentMap{
			"$.debug": {yaml.HeadComment(" enable verbose logging and diagnostics")},

			"$.tokens":         {yaml.HeadComment("")},
			"$.settings":       {yaml.HeadComment("")},
			"$.authentication": {yaml.HeadComment("")},

			"$.tokens.secret":     {yaml.HeadComment(" server secret for signing auth tokens; auto-generated if empty")},
			"$.tokens.openrouter": {yaml.HeadComment(" openrouter.ai api token (required)")},
			"$.tokens.exa":        {yaml.HeadComment(" exa search api token (optional; used by search tools)")},
			"$.tokens.github":     {yaml.HeadComment(" github api token (optional; used by search tools)")},

			"$.server.port": {yaml.HeadComment(" port to serve whiskr on (required; default 3443)")},

			"$.settings.cleanup":          {yaml.HeadComment(" normalize unicode in assistant output (optional; default: true)")},
			"$.settings.timeout":          {yaml.HeadComment(" the http timeout to use for completion requests in seconds (optional; default: 300s)")},
			"$.settings.refresh-interval": {yaml.HeadComment(" the interval in which the model list is refreshed in minutes (optional; default: 30m)")},

			"$.models.title-model":      {yaml.HeadComment(" model used to generate titles (needs to have structured output support; default: google/gemini-2.5-flash-lite)")},
			"$.models.image-generation": {yaml.HeadComment(" allow image generation (optional; default: true)")},
			"$.models.transformation":   {yaml.HeadComment(" what transformation method to use for too long contexts (optional; default: middle-out)")},
			"$.models.filters":          {yaml.HeadComment(" boolean expression to filter available models (optional; fields: `price`, `slug`, `name`, `tags`, `created`; operators: `<`, `>`, `==`, `!=`, `~` (contains), `^` (starts-with), `$` (ends-with); Logic: `&&`, `||`, `!`, `( )`)")},

			"$.ui.reduced-motion": {yaml.HeadComment(" disables things like the floating stars in the background (optional; default: false)")},

			"$.authentication.enabled": {yaml.HeadComment(" require login with username and password")},
			"$.authentication.users":   {yaml.HeadComment(" list of users with bcrypt password hashes")},
		}
	)

	e.dmx.RLock()
	err := yaml.NewEncoder(&buffer, yaml.WithComment(comments)).Encode(e)
	e.dmx.RUnlock()

	if err != nil {
		return err
	}

	body := bytes.ReplaceAll(buffer.Bytes(), []byte("#\n"), []byte("\n"))

	e.fmx.Lock()
	defer e.fmx.Unlock()

	return os.WriteFile("config.yml", body, 0644)
}

func CreateSecret(length int) (string, error) {
	key := make([]byte, length)

	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(key), nil
}
