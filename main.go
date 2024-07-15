package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func main() {
	// Command-line argument for the values.yaml file path
	valuesFilePath := flag.String("f", "values.yaml", "path to the values.yaml file")
	flag.Parse()

	// Read the values.yaml file
	data, err := os.ReadFile(*valuesFilePath)
	if err != nil {
		log.Fatalf("failed to read file: %v", err)
	}

	// Define regex to find SSM placeholders
	re := regexp.MustCompile(`{{\s*ssm\s+([^ ]+)\s+([^ ]+)\s*}}`)

	// Find all matches
	matches := re.FindAllStringSubmatch(string(data), -1)

	// Group parameters by region
	parametersByRegion := make(map[string][]string)
	for _, match := range matches {
		paramPath := match[1]
		region := match[2]
		parametersByRegion[region] = append(parametersByRegion[region], paramPath)
	}

	// Load default AWS config
	defaultCfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Map to store parameter values
	paramValues := make(map[string]string)

	// Retrieve parameters for each region
	for region, paramPaths := range parametersByRegion {
		// Override region if necessary
		cfg := defaultCfg.Copy()
		cfg.Region = region
		ssmClient := ssm.NewFromConfig(cfg)

		// Get the parameter values from SSM
		paramValuesInRegion, err := getSSMParameters(ssmClient, paramPaths)
		if err != nil {
			log.Fatalf("failed to get SSM parameters: %v", err)
		}

		// Add retrieved values to the map
		for key, value := range paramValuesInRegion {
			paramValues[key] = value
		}
	}

	// Replace each match with the actual SSM parameter value
	for _, match := range matches {
		paramPath := match[1]
		placeholder := match[0]
		if paramValue, ok := paramValues[paramPath]; ok {
			data = []byte(strings.ReplaceAll(string(data), placeholder, paramValue))
		}
	}

	// Print the modified values.yaml
	fmt.Println(string(data))
}

func getSSMParameters(client *ssm.Client, paramPaths []string) (map[string]string, error) {
	input := &ssm.GetParametersInput{
		Names:          paramPaths,
		WithDecryption: aws.Bool(true),
	}

	result, err := client.GetParameters(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	paramValues := make(map[string]string)
	for _, param := range result.Parameters {
		paramValues[*param.Name] = *param.Value
	}

	return paramValues, nil
}
