package main

import (
	"fmt"
	"strconv"
	"strings"
)

type updaterConfig struct {
	clusterARN    string
	serviceName   string
	hostedZoneID  string
	dnsName       string
	dnsTTLSeconds int64
}

func loadConfig(getenv func(string) string) (updaterConfig, error) {
	clusterARN, err := requiredEnv(getenv, "CLUSTER_ARN")
	if err != nil {
		return updaterConfig{}, err
	}
	if !isECSClusterARN(clusterARN) {
		return updaterConfig{}, fmt.Errorf("CLUSTER_ARN must be an ecs cluster arn")
	}

	serviceName, err := requiredEnv(getenv, "SERVICE_NAME")
	if err != nil {
		return updaterConfig{}, err
	}
	if strings.ContainsAny(serviceName, " \t\r\n") {
		return updaterConfig{}, fmt.Errorf("SERVICE_NAME must not contain whitespace")
	}

	hostedZoneID, err := requiredEnv(getenv, "HOSTED_ZONE_ID")
	if err != nil {
		return updaterConfig{}, err
	}
	if strings.ContainsAny(hostedZoneID, " \t\r\n") {
		return updaterConfig{}, fmt.Errorf("HOSTED_ZONE_ID must not contain whitespace")
	}

	dnsName, err := requiredEnv(getenv, "DNS_NAME")
	if err != nil {
		return updaterConfig{}, err
	}
	dnsName = strings.TrimSuffix(strings.ToLower(dnsName), ".")
	if !isDNSName(dnsName) {
		return updaterConfig{}, fmt.Errorf("DNS_NAME must be a valid dns name")
	}

	dnsTTL, err := requiredEnv(getenv, "DNS_TTL")
	if err != nil {
		return updaterConfig{}, err
	}
	dnsTTLSeconds, err := strconv.ParseInt(dnsTTL, 10, 64)
	if err != nil || dnsTTLSeconds <= 0 {
		return updaterConfig{}, fmt.Errorf("DNS_TTL must be a positive integer")
	}

	return updaterConfig{
		clusterARN:    clusterARN,
		serviceName:   serviceName,
		hostedZoneID:  hostedZoneID,
		dnsName:       dnsName,
		dnsTTLSeconds: dnsTTLSeconds,
	}, nil
}

func requiredEnv(getenv func(string) string, name string) (string, error) {
	value := strings.TrimSpace(getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}

	return value, nil
}

func isECSClusterARN(value string) bool {
	parts := strings.SplitN(value, ":", 6)
	return len(parts) == 6 && parts[0] == "arn" && parts[2] == "ecs" &&
		parts[3] != "" && parts[4] != "" && strings.HasPrefix(parts[5], "cluster/")
}

func isDNSName(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}

	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			isLetter := character >= 'a' && character <= 'z'
			isDigit := character >= '0' && character <= '9'
			if !isLetter && !isDigit && character != '-' {
				return false
			}
		}
	}

	return true
}
