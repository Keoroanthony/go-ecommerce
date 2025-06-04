package notifier

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"

	"github.com/Keoroanthony/go-ecommerce/configs"
)

func SendEmail(recipientEmail string, customerName string, orderID uint, totalAmount float64) error {
	cfg := config.LoadEmailConfig()

	
	awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(cfg.AWSRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, "")),
	)
	if err != nil {

		log.Printf("Failed to load AWS SDK config for email to %s (order %d): %v", recipientEmail, orderID, err)
		return fmt.Errorf("failed to load AWS SDK config: %w", err)
	}

	client := ses.NewFromConfig(awsCfg)

	if cfg.SenderEmail == "" {
		return fmt.Errorf("sender email address is not configured in environment variables")
	}
	if recipientEmail == "" {
		return fmt.Errorf("recipient email address is empty")
	}

	subject := fmt.Sprintf("Order #%d Confirmation - Thank You for Your Purchase!", orderID)

	totalAmountStr := strconv.FormatFloat(totalAmount, 'f', 2, 64)

	bodyHTML := fmt.Sprintf(`
        <html>
        <body>
            <p>Dear %s,</p>
            <p>Thank you for your order! Your order #%d has been successfully placed.</p>
            <p><strong>Order Details:</strong></p>
            <ul>
                <li>Order ID: %d</li>
                <li>Total Amount: KES %s</li>
            </ul>
            <p>We'll send you another email when your order ships.</p>
            <p>Best regards,</p>
            <p>Your E-commerce Team</p>
        </body>
        </html>`, customerName, orderID, orderID, totalAmountStr)

	bodyText := fmt.Sprintf(
		"Dear %s,\n\nThank you for your order! Your order #%d has been successfully placed.\n\n"+
			"Order Details:\nOrder ID: %d\nTotal Amount: KES %s\n\n"+ // Use totalAmountStr here
			"We'll send you another email when your order ships.\n\nBest regards,\nYour E-commerce Team",
		customerName, orderID, orderID, totalAmountStr)

	input := &ses.SendEmailInput{
		Source: aws.String(cfg.SenderEmail),
		Destination: &types.Destination{
			ToAddresses: []string{recipientEmail},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String(subject),
			},
			Body: &types.Body{
				Html: &types.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(bodyHTML),
				},
				Text: &types.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(bodyText),
				},
			},
		},
	}

	_, err = client.SendEmail(context.TODO(), input)
	if err != nil {
		log.Printf("Failed to send email for order %d to %s: %v", orderID, recipientEmail, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Order confirmation email sent successfully for order %d to %s", orderID, recipientEmail)
	return nil
}