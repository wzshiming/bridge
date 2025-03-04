package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/bridge/logger"
)

func LoadConfigWithArgs(listens []string, dials []string) ([]Chain, error) {
	if len(dials) > 0 && len(listens) > 0 && dials[0] == "-" {
		proxies := strings.Split(listens[0], "|")
		if len(proxies) == 1 {
			network, address, _ := scheme.SplitSchemeAddr(proxies[0])
			if network == "tcp" {
				proxies = anyProxy(address)
			}
		}
		listens[0] = strings.Join(proxies, "|")
	}
	data := struct {
		Bind  []string `json:"bind"`
		Proxy []string `json:"proxy"`
	}{
		Bind:  listens,
		Proxy: dials,
	}
	rawJson, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	conf := Chain{}
	err = json.Unmarshal(rawJson, &conf)
	if err != nil {
		return nil, err
	}
	err = conf.Verification()
	if err != nil {
		return nil, err
	}
	return []Chain{conf}, nil
}

func anyProxy(address string) []string {
	return []string{"http://" + address, "socks5://" + address, "socks4://" + address, "ssh://" + address}
}

func LoadConfig(configs ...string) ([]Chain, error) {
	tasks := []Chain{}
	for _, confPath := range configs {
		data, err := os.ReadFile(confPath)
		if err != nil {
			logger.Std.Error("LoadConfig", "err", err, "path", confPath)
			continue
		}
		conf := Config{}
		err = json.Unmarshal(data, &conf)
		if err != nil {
			return nil, err
		}
		for _, ch := range conf.Chains {
			err := ch.Verification()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", confPath, err)
			}
			tasks = append(tasks, ch)
		}
	}
	return tasks, nil
}

type Config struct {
	Chains []Chain `json:"chains"`
}

type Chain struct {
	Bind        []Node        `json:"bind"`
	Proxy       []Node        `json:"proxy"`
	IdleTimeout time.Duration `json:"idle_timeout"`
}

func (c Chain) Verification() error {
	if len(c.Proxy) == 0 {
		return fmt.Errorf("must has proxy")
	}
	return nil
}

func (c Chain) Unique() string {
	d, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(d)
}

type Node struct {
	LB []string `json:"lb"`
}

func (m Node) MarshalJSON() ([]byte, error) {
	if len(m.LB) == 1 {
		return json.Marshal(m.LB[0])
	}
	type node Node
	return json.Marshal(node(m))
}

func (m *Node) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("node: UnmarshalJSON on nil pointer")
	}
	if len(data) > 0 {
		switch data[0] {
		case '"':
			str, err := strconv.Unquote(string(data))
			if err != nil {
				return err
			}
			m.LB = strings.Split(str, "|")
			return nil
		case '[':
			return json.Unmarshal(data, &m.LB)
		}
	}
	type node Node
	return json.Unmarshal(data, (*node)(m))
}
