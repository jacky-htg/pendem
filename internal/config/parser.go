package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

type ConfigParser struct {
	data map[string]map[string]string
}

func NewConfigParser() *ConfigParser {
	return &ConfigParser{
		data: make(map[string]map[string]string),
	}
}

func (p *ConfigParser) Parse(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentSection string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section: [name]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			if p.data[currentSection] == nil {
				p.data[currentSection] = make(map[string]string)
			}
			continue
		}

		// Key = value
		if currentSection != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				p.data[currentSection][key] = value
			}
		}
	}

	return scanner.Err()
}

func (p *ConfigParser) GetString(section, key string) (string, bool) {
	if sec, ok := p.data[section]; ok {
		if val, ok := sec[key]; ok {
			return val, true
		}
	}
	return "", false
}

func (p *ConfigParser) GetInt(section, key string) (int, bool) {
	if val, ok := p.GetString(section, key); ok {
		var result int
		fmt.Sscanf(val, "%d", &result)
		return result, true
	}
	return 0, false
}

func (p *ConfigParser) GetDuration(section, key string) (time.Duration, bool) {
	if val, ok := p.GetString(section, key); ok {
		// Support: 1s, 1m, 1h, 1d
		if d, err := time.ParseDuration(val); err == nil {
			return d, true
		}
		// Support: 1GB, 1MB, 1KB
		if strings.HasSuffix(val, "GB") {
			// ...
		}
	}
	return 0, false
}

func (p *ConfigParser) GetBool(section, key string) (bool, bool) {
	if val, ok := p.GetString(section, key); ok {
		return val == "true" || val == "yes" || val == "1", true
	}
	return false, false
}

func (p *ConfigParser) GetBytes(section, key string) (int64, bool) {
	if val, ok := p.GetString(section, key); ok {
		val = strings.ToUpper(val)
		var multiplier int64 = 1

		if strings.HasSuffix(val, "GB") {
			multiplier = 1024 * 1024 * 1024
			val = strings.TrimSuffix(val, "GB")
		} else if strings.HasSuffix(val, "MB") {
			multiplier = 1024 * 1024
			val = strings.TrimSuffix(val, "MB")
		} else if strings.HasSuffix(val, "KB") {
			multiplier = 1024
			val = strings.TrimSuffix(val, "KB")
		}

		var result int64
		fmt.Sscanf(strings.TrimSpace(val), "%d", &result)
		return result * multiplier, true
	}
	return 0, false
}
