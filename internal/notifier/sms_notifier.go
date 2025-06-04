package notifier

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url" 
    "strings"

	"github.com/Keoroanthony/go-ecommerce/configs"
)

type SMSResponse struct {
	SMSMessageData struct {
		Message string `json:"Message"`
		Recipients []struct {
			StatusCode int    `json:"statusCode"`
			Number     string `json:"number"`
			Cost       string `json:"cost"`
			Status     string `json:"status"`
			MessageID  string `json:"messageId"`
		} `json:"Recipients"`
	} `json:"SMSMessageData"`
}

func SendSMS(toPhoneNumber string, orderID uint, totalAmount float64) error {

	cfg := config.LoadAfricaTalkingConfig()

	message := fmt.Sprintf("Your order #%d has been successfully placed! Total: KES %.2f. Thank you for shopping with us!", orderID, totalAmount)

	data := url.Values{}
	data.Set("username", cfg.Username)
	data.Set("to", toPhoneNumber)
	data.Set("message", message)
	data.Set("from", cfg.SenderID)

	client := &http.Client{}
	req, err := http.NewRequest("POST", cfg.SMSURL, strings.NewReader(data.Encode()))

	if err != nil {
		return fmt.Errorf("failed to create SMS request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("apikey", cfg.APIKey)

	resp, err := client.Do(req)

	if err != nil {
		log.Printf("SMS send failed to %s for order %d: %v\n", toPhoneNumber, orderID, err)
		return fmt.Errorf("SMS send failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var smsResp SMSResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&smsResp); decodeErr == nil {
			log.Printf("SMS API returned error for %s (order %d): Status %d, Message: %s\n", toPhoneNumber, orderID, resp.StatusCode, smsResp.SMSMessageData.Message)
		} else {
			log.Printf("SMS API returned non-success status %d for %s (order %d) and failed to decode response: %v\n", resp.StatusCode, toPhoneNumber, orderID, decodeErr)
		}
		return fmt.Errorf("SMS API returned non-success status: %d", resp.StatusCode)
	}

	var smsResp SMSResponse
	if err := json.NewDecoder(resp.Body).Decode(&smsResp); err != nil {
		log.Printf("Failed to decode SMS response for %s (order %d): %v\n", toPhoneNumber, orderID, err)
		return fmt.Errorf("failed to decode SMS response: %w", err)
	}

	log.Printf("SMS sent successfully to %s for order %d. Message: %s\n", toPhoneNumber, orderID, smsResp.SMSMessageData.Message)
	return nil
}