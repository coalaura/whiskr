package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"

	"github.com/goccy/go-yaml"
)

type EnvTokens struct {
	Secret     string `yaml:"secret"`
	OpenRouter string `yaml:"openrouter"`
	Exa        string `yaml:"exa"`
	GitHub     string `yaml:"github"`
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

	filters FilterList
}

type EnvUI struct {
	ReducedMotion bool `yaml:"reduced-motion"`
}

type EnvUser struct {
	ID       string `yaml:"id"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type EnvAuthentication struct {
	lookup map[string]*EnvUser

	Enabled bool       `yaml:"enabled"`
	Users   []*EnvUser `yaml:"users"`
}

type Environment struct {
	Debug          bool              `yaml:"debug"`
	Tokens         EnvTokens         `yaml:"tokens"`
	Settings       EnvSettings       `yaml:"settings"`
	Models         EnvModels         `yaml:"models"`
	UI             EnvUI             `yaml:"ui"`
	Authentication EnvAuthentication `yaml:"authentication"`
}

var env = Environment{
	// defaults
	Settings: EnvSettings{
		CleanContent:    true,
		Timeout:         1200,
		RefreshInterval: 30,
	},
	Models: EnvModels{
		ImageGeneration: true,
	},
}

func init() {
	file, err := os.OpenFile("config.yml", os.O_RDONLY, 0)
	log.MustFail(err)

	defer file.Close()

	err = yaml.NewDecoder(file).Decode(&env)
	log.MustFail(err)

	log.MustFail(env.Init())
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

	for i, user := range e.Authentication.Users {
		if user.ID == "" {
			log.Warnf("User %q has no id, generating new\n", user.Username)

			id, err := CreateSecret(16)
			if err != nil {
				return err
			}

			user.ID = id

			e.Authentication.Users[i] = user

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

			"$.settings.cleanup":          {yaml.HeadComment(" normalize unicode in assistant output (optional; default: true)")},
			"$.settings.timeout":          {yaml.HeadComment(" the http timeout to use for completion requests in seconds (optional; default: 300s)")},
			"$.settings.refresh-interval": {yaml.HeadComment(" the interval in which the model list is refreshed in minutes (optional; default: 30m)")},

			"$.models.title-model":      {yaml.HeadComment(" model used to generate titles (needs to have structured output support; default: google/gemini-2.5-flash-lite)")},
			"$.models.image-generation": {yaml.HeadComment(" allow image generation (optional; default: true)")},
			"$.models.transformation":   {yaml.HeadComment(" what transformation method to use for too long contexts (optional; default: middle-out)")},
			"$.models.filters":          {yaml.HeadComment(" filters to apply to the model list comma separated (optional; fields: `price`, `id`, `name`; operators: `<` (less than), `>` (greater than), `=` (equals), `~` (contains), `^` (starts-with), `$` (ends-with))")},

			"$.ui.reduced-motion": {yaml.HeadComment(" disables things like the floating stars in the background (optional; default: false)")},

			"$.authentication.enabled": {yaml.HeadComment(" require login with username and password")},
			"$.authentication.users":   {yaml.HeadComment(" list of users with bcrypt password hashes")},
		}
	)

	err := yaml.NewEncoder(&buffer, yaml.WithComment(comments)).Encode(e)
	if err != nil {
		return err
	}

	body := bytes.ReplaceAll(buffer.Bytes(), []byte("#\n"), []byte("\n"))

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
