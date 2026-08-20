package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"golang.org/x/crypto/bcrypt"
)

// LLM api types
const (
	APIOpenRouter = "openrouter"
	APIOpenAI     = "openai"
)

// gost:preserve-layout
type EnvTokens struct {
	Secret     string `yaml:"secret"`
	OpenRouter string `yaml:"openrouter"`
	OpenAI     string `yaml:"openai"`
	Tavily     string `yaml:"tavily"`
	GitHub     string `yaml:"github"`
}

// gost:preserve-layout
type EnvServer struct {
	Port int64 `yaml:"port"`
}

// gost:preserve-layout
type EnvSettings struct {
	CleanContent    bool  `yaml:"cleanup"`
	Timeout         int64 `yaml:"timeout"`
	RefreshInterval int64 `yaml:"refresh-interval"`
}

// gost:preserve-layout
type EnvLLM struct {
	API     string `yaml:"api"`
	BaseURL string `yaml:"base-url"`
}

// gost:preserve-layout
type EnvModels struct {
	TitleModel      string `yaml:"title-model"`
	ImageGeneration bool   `yaml:"image-generation"`
	TextToSpeech    bool   `yaml:"text-to-speech"`
	Transformation  string `yaml:"transformation"`
	Filters         string `yaml:"filters"`

	filters *Filters
}

// gost:preserve-layout
type EnvUI struct {
	ReducedMotion bool `yaml:"reduced-motion"`
}

// gost:preserve-layout
type EnvUser struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// gost:preserve-layout
type EnvAuthentication struct {
	lookup map[string]*EnvUser

	Enabled bool       `yaml:"enabled"`
	Users   []*EnvUser `yaml:"users"`
}

// gost:preserve-layout
type EnvProxy struct {
	transport http.RoundTripper

	Name  string `yaml:"name"`
	Host  string `yaml:"host"`
	Token string `yaml:"token"`
}

// gost:preserve-layout
type Environment struct {
	dmx sync.RWMutex // data mutex
	fmx sync.Mutex   // file mutex

	Debug          bool              `yaml:"debug"`
	Tokens         EnvTokens         `yaml:"tokens"`
	Server         EnvServer         `yaml:"server"`
	Proxies        []EnvProxy        `yaml:"proxies"`
	Settings       EnvSettings       `yaml:"settings"`
	LLM            EnvLLM            `yaml:"llm"`
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
		LLM: EnvLLM{
			API: APIOpenRouter,
		},
		Models: EnvModels{
			ImageGeneration: true,
			TextToSpeech:    true,
		},
	}

	file, err := os.OpenFile(path.Config, os.O_RDONLY, 0)
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

// IsOpenAI reports whether the configured llm api is an openai-compatible endpoint.
func (e *Environment) IsOpenAI() bool {
	return e.LLM.API == APIOpenAI
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

	// print if text-to-speech is enabled
	if e.Models.TextToSpeech {
		log.Warnln("Text-to-speech enabled")
	} else {
		log.Warnln("Text-to-speech disabled")
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

	// normalize the llm api type
	if e.LLM.API == "" {
		e.LLM.API = APIOpenRouter
	}

	// normalize the llm base url
	if e.LLM.BaseURL == "" {
		e.LLM.BaseURL = "https://openrouter.ai/api/v1/"
	}

	if e.LLM.API != APIOpenRouter && e.LLM.API != APIOpenAI {
		return fmt.Errorf("invalid llm.api %q (must be %q or %q)", e.LLM.API, APIOpenRouter, APIOpenAI)
	}

	// check the api token for the selected llm api
	switch e.LLM.API {
	case APIOpenAI:
		if e.Tokens.OpenAI == "" {
			return errors.New("missing tokens.openai")
		}
	default:
		if e.Tokens.OpenRouter == "" {
			return errors.New("missing tokens.openrouter")
		}
	}

	if e.LLM.API == APIOpenAI {
		log.Warnf("Using OpenAI compatible endpoint: %s\n", e.LLM.BaseURL)
	}

	// check if tavily token is set
	if e.Tokens.Tavily == "" {
		log.Warnln("Missing token.tavily, web search unavailable")
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

	// make it harder to disable auth accidentally
	if !e.Authentication.Enabled && len(e.Authentication.Users) > 0 {
		return errors.New("authentication disabled but users defined")
	}

	// validate proxy entries
	proxyNames := make(map[string]struct{}, len(e.Proxies))

	for i := range e.Proxies {
		proxy := &e.Proxies[i]

		if proxy.Name == "" {
			return errors.New("proxy missing name")
		}

		if proxy.Host == "" {
			return fmt.Errorf("proxy %q missing host", proxy.Name)
		}

		if proxy.Token == "" {
			return fmt.Errorf("proxy %q missing auth", proxy.Name)
		}

		if _, ok := proxyNames[proxy.Name]; ok {
			return fmt.Errorf("duplicate proxy name %q", proxy.Name)
		}

		proxyNames[proxy.Name] = struct{}{}

		proxy.transport = NewProxyTransport(proxy.Host, proxy.Token)
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
			"$.server":         {yaml.HeadComment("")},
			"$.settings":       {yaml.HeadComment("")},
			"$.proxies":        {yaml.HeadComment("")},
			"$.llm":            {yaml.HeadComment("")},
			"$.models":         {yaml.HeadComment("")},
			"$.ui":             {yaml.HeadComment("")},
			"$.authentication": {yaml.HeadComment("")},

			"$.tokens.secret":     {yaml.HeadComment(" server secret for signing auth tokens; auto-generated if empty")},
			"$.tokens.openrouter": {yaml.HeadComment(" openrouter.ai api token (used when llm.api is openrouter)")},
			"$.tokens.openai":     {yaml.HeadComment(" openai-compatible api token (used when llm.api is openai)")},
			"$.tokens.tavily":     {yaml.HeadComment(" tavily search api token (optional; used by search tools)")},
			"$.tokens.github":     {yaml.HeadComment(" github api token (optional; used by search tools)")},

			"$.server.port": {yaml.HeadComment(" port to serve whiskr on (required; default 3443)")},

			"$.settings.cleanup":          {yaml.HeadComment(" normalize unicode in assistant output (optional; default: true)")},
			"$.settings.timeout":          {yaml.HeadComment(" the http timeout to use for completion requests in seconds (optional; default: 1200s)")},
			"$.settings.refresh-interval": {yaml.HeadComment(" the interval in which the model list is refreshed in minutes (optional; default: 30m)")},
			"$.settings.statistics":       {yaml.HeadComment(" track non-identifying completion stats in sqlite (optional; default: true)")},

			"$.llm.api":      {yaml.HeadComment(" llm api type: openrouter (default) or openai (openai-compatible endpoint)")},
			"$.llm.base-url": {yaml.HeadComment(" override the api base url (optional; defaults to https://openrouter.ai/api/v1 or https://api.openai.com/v1)")},

			"$.models.title-model":      {yaml.HeadComment(" model used to generate titles (needs to have structured output support; set to \"-\" to disable title; default: google/gemini-2.5-flash-lite)")},
			"$.models.image-generation": {yaml.HeadComment(" allow image generation (optional; default: true)")},
			"$.models.text-to-speech":   {yaml.HeadComment(" allow text to speech (optional; default: true)")},
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

	return os.WriteFile(path.Config, body, 0644)
}

func CreateSecret(length int) (string, error) {
	key := make([]byte, length)

	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(key), nil
}
