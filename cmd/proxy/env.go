package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/goccy/go-yaml"
)

// gost:preserve-layout
type EnvServer struct {
	Port  int    `yaml:"port"`
	Token string `yaml:"token"`
}

// gost:preserve-layout
type Environment struct {
	dmx sync.RWMutex // data mutex
	fmx sync.Mutex   // file mutex

	Server EnvServer `yaml:"server"`
}

func LoadEnv() (*Environment, error) {
	// defaults
	cfg := &Environment{
		Server: EnvServer{
			Port: 4334,
		},
	}

	file, err := os.OpenFile("config.yml", os.O_RDONLY, 0)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		defer file.Close()

		err = yaml.NewDecoder(file).Decode(cfg)
		if err != nil {
			return nil, err
		}
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
	// check if token is set
	if e.Server.Token != "" {
		return nil
	}

	log.Warnln("Missing server.token, generating new")

	secret, err := CreateToken()
	if err != nil {
		return err
	}

	e.Server.Token = secret

	err = e.Store()
	if err != nil {
		return err
	}

	log.Println("Updated config.yml")

	return nil
}

func (e *Environment) Store() error {
	var (
		buffer   bytes.Buffer
		comments = yaml.CommentMap{
			"$.server.port":  {yaml.HeadComment(" port to run the proxy on (required; default 4334)")},
			"$.server.token": {yaml.HeadComment(" token for authenticating proxy requests; auto-generated if empty")},
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

func CreateToken() (string, error) {
	random := make([]byte, 25)

	_, err := io.ReadFull(rand.Reader, random)
	if err != nil {
		return "", err
	}

	buf := make([]byte, 4+hex.EncodedLen(25))

	copy(buf, "wsk-")

	hex.Encode(buf[4:], random)

	buf[20] = '-'
	buf[37] = '-'

	return string(buf), nil
}
