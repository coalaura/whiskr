package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"

	"github.com/goccy/go-yaml"
)

type EnvTokens struct {
	Secret     string `json:"secret"`
	OpenRouter string `json:"openrouter"`
	Exa        string `json:"exa"`
}

type EnvSettings struct {
	CleanContent  bool `json:"cleanup"`
	MaxIterations uint `json:"iterations"`
}

type EnvUser struct {
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
		CleanContent:  true,
		MaxIterations: 3,
	},
}

func init() {
	file, err := os.OpenFile("config.yml", os.O_RDONLY, 0)
	log.MustPanic(err)

	defer file.Close()

	err = yaml.NewDecoder(file).Decode(&env)
	log.MustPanic(err)

	log.MustPanic(env.Init())
}

func (e *Environment) Init() error {
	// print if debug is enabled
	if e.Debug {
		log.Warning("Debug mode enabled")
	}

	// check max iterations
	e.Settings.MaxIterations = max(e.Settings.MaxIterations, 1)

	// check if server secret is set
	if e.Tokens.Secret == "" {
		log.Warning("Missing tokens.secret, generating new...")

		key := make([]byte, 32)

		_, err := io.ReadFull(rand.Reader, key)
		if err != nil {
			return err
		}

		e.Tokens.Secret = base64.StdEncoding.EncodeToString(key)

		err = e.Store()
		if err != nil {
			return err
		}

		log.Info("Stored new tokens.secret")
	}

	// check if openrouter token is set
	if e.Tokens.OpenRouter == "" {
		return errors.New("missing tokens.openrouter")
	}

	// check if exa token is set
	if e.Tokens.Exa == "" {
		log.Warning("Missing token.exa, web search unavailable")
	}

	// create user lookup map
	e.Authentication.lookup = make(map[string]*EnvUser)

	for _, user := range e.Authentication.Users {
		e.Authentication.lookup[user.Username] = user
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

			"$.settings.cleanup":    {yaml.HeadComment(" normalize unicode in assistant output (optional; default: true)")},
			"$.settings.iterations": {yaml.HeadComment(" max model turns per request (optional; default: 3)")},

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
