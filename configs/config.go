package config

import (
	"os"
)

type AfricaTalkingConfig struct {
	Username string
	APIKey   string
	SMSURL   string 
	SenderID string 
}

type EmailConfig struct {
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string
	SenderEmail        string
}

func LoadAfricaTalkingConfig() AfricaTalkingConfig {
	return AfricaTalkingConfig{
		Username: os.Getenv("AT_USERNAME"),
		APIKey:   os.Getenv("AT_API_KEY"),
		SMSURL:   getEnvOrDefault("AT_SMS_URL", "https://api.sandbox.africastalking.com/version1/messaging"), // Sandbox URL
		SenderID: getEnvOrDefault("AT_SENDER_ID", "AFRICASTKNG"), // Default sandbox sender ID
	}
}

func LoadEmailConfig() EmailConfig {
	return EmailConfig{
		AWSAccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSRegion:          getEnvOrDefault("AWS_REGION", "us-east-1"),
		SenderEmail:        os.Getenv("AWS_SENDER_ADDRESS"),
	}
}


func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}