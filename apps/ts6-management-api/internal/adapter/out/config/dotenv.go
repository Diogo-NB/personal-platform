package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type environmentLookup func(string) (string, bool)
type environmentSetter func(string, string) error

func loadDotEnv(path string, lookup environmentLookup, set environmentSetter) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("config: open dotenv: %w", err)
	}
	defer file.Close()

	values, err := parseDotEnv(file)
	if err != nil {
		return err
	}
	for key, value := range values {
		if _, exists := lookup(key); exists {
			continue
		}
		if err := set(key, value); err != nil {
			return fmt.Errorf("config: set dotenv variable %q: %w", key, err)
		}
	}

	return nil
}

func parseDotEnv(reader io.Reader) (map[string]string, error) {
	values := map[string]string{}
	scanner := bufio.NewScanner(reader)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, rawValue, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || !isEnvironmentKey(key) {
			return nil, fmt.Errorf("config: invalid dotenv entry on line %d", lineNumber)
		}

		value, err := parseDotEnvValue(rawValue)
		if err != nil {
			return nil, fmt.Errorf("config: invalid dotenv value on line %d: %w", lineNumber, err)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("config: read dotenv: %w", err)
	}

	return values, nil
}

func parseDotEnvValue(rawValue string) (string, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return value, nil
	}

	quote := value[0]
	if quote != '\'' && quote != '"' {
		return value, nil
	}
	if len(value) < 2 || value[len(value)-1] != quote {
		return "", errors.New("unterminated quote")
	}
	if quote == '\'' {
		return value[1 : len(value)-1], nil
	}

	return strconv.Unquote(value)
}

func isEnvironmentKey(key string) bool {
	if key == "" || !isEnvironmentKeyStart(key[0]) {
		return false
	}
	for index := 1; index < len(key); index++ {
		character := key[index]
		if !isEnvironmentKeyStart(character) && (character < '0' || character > '9') {
			return false
		}
	}

	return true
}

func isEnvironmentKeyStart(character byte) bool {
	isUppercase := character >= 'A' && character <= 'Z'
	isLowercase := character >= 'a' && character <= 'z'
	return character == '_' || isUppercase || isLowercase
}
