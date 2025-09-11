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
	Secret     string `json:"secret"`
	OpenRouter string `json:"openrouter"`
	Exa        string `json:"exa"`
	GitHub     string `json:"github"`
}

type EnvSettings struct {
	CleanContent    bool   `json:"cleanup"`
	TitleModel      string `json:"title-model"`
	ImageGeneration bool   `json:"image-generation"`
}

type EnvUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type EnvAuthentication struct {
	lookup map[string]*EnvUser

	Enabled bool       `json:"enabled"`
	Users   []*EnvUser `json:"users"`
}

type Environment struct {
	Debug          bool              `json:"debug"`
	Tokens         EnvTokens         `json:"tokens"`
	Settings       EnvSettings       `json:"settings"`
	Authentication EnvAuthentication `json:"authentication"`
}

var env = Environment{
	// defaults
	Settings: EnvSettings{
		CleanContent:    true,
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
	if e.Settings.ImageGeneration {
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
	if e.Settings.TitleModel == "" {
		e.Settings.TitleModel = "google/gemini-2.5-flash-lite"
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
			"$.settings.title-model":      {yaml.HeadComment(" model used to generate titles (needs to have structured output support; default: google/gemini-2.5-flash-lite)")},
			"$.settings.image-generation": {yaml.HeadComment(" allow image generation (optional; default: true)")},

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
