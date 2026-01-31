package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

var (
	sesSenderEmail string
	sesClient      *ses.Client
	logger         *slog.Logger
)

// LicenseDeliveryMessage represents the message from license generator
type LicenseDeliveryMessage struct {
	Email          string `json:"email"`
	License        string `json:"license"`
	ExpirationDate string `json:"expirationDate"`
	ValidDays      int    `json:"validDays"`
}

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("License sender lambda initializing",
		"component", "license-sender",
		"stage", "init")

	// Get environment variables
	sesSenderEmail = os.Getenv("SES_SENDER_EMAIL")
	if sesSenderEmail == "" {
		sesSenderEmail = "noreply@breachline.app"
		logger.Info("Using default sender email",
			"email", sesSenderEmail,
			"stage", "init")
	} else {
		logger.Info("Sender email configured",
			"email", sesSenderEmail,
			"stage", "init")
	}

	// Initialize AWS SES client
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Error("Failed to load AWS config", "error", err, "stage", "init")
	} else {
		sesClient = ses.NewFromConfig(cfg)
		logger.Info("AWS SES client initialized", "stage", "init")
	}

	logger.Info("Initialization complete", "stage", "init")
}

// Handler is the Lambda function handler for SNS events
func Handler(ctx context.Context, snsEvent events.SNSEvent) error {
	logger.Info("Received SNS event",
		"record_count", len(snsEvent.Records))

	// Process each SNS record (usually just one)
	for _, record := range snsEvent.Records {
		logger.Info("Processing SNS message",
			"message_id", record.SNS.MessageID,
			"subject", record.SNS.Subject)

		// Parse the SNS message body
		var deliveryMsg LicenseDeliveryMessage
		if err := json.Unmarshal([]byte(record.SNS.Message), &deliveryMsg); err != nil {
			logger.Error("Failed to parse SNS message",
				"error", err,
				"message", record.SNS.Message)
			return fmt.Errorf("invalid SNS message: %w", err)
		}

		logger.Info("License delivery request parsed",
			"email", deliveryMsg.Email,
			"validity_days", deliveryMsg.ValidDays)

		if err := sendLicenseEmail(ctx, deliveryMsg); err != nil {
			logger.Error("Failed to send license email",
				"error", err,
				"email", deliveryMsg.Email)
			return err
		}

		logger.Info("License email sent successfully",
			"email", deliveryMsg.Email)
	}

	return nil
}

// sendLicenseEmail sends the license to the customer via SES
func sendLicenseEmail(ctx context.Context, msg LicenseDeliveryMessage) error {
	logger.Info("Sending license email",
		"recipient", msg.Email,
		"validity_days", msg.ValidDays,
		"expiration", msg.ExpirationDate)

	// Use the license as-is (it's already base64 encoded from the generator)
	licenseBytes := []byte(msg.License)
	logger.Info("License prepared for attachment",
		"size_bytes", len(licenseBytes))

	// Create email subject
	subject := "Your BreachLine Premium License"
	logger.Info("Email prepared",
		"subject", subject)

	// Create email body with instructions
	bodyText := fmt.Sprintf(`Thank you for your purchase of BreachLine Premium!

Your license is attached to this email as a file. This license is valid for %d days, expiring on %s.

To import your license into BreachLine:

1. Open BreachLine on your computer
2. Go to File → Import License File
3. Select the license file attached to this email (breachline-license.lic)

Your Premium features will be activated immediately!

If you have any questions or need assistance, please don't hesitate to contact us.

Thank you for choosing BreachLine!

Best regards,
The BreachLine Team
`, msg.ValidDays, msg.ExpirationDate)

	bodyHTML := fmt.Sprintf(`
<html>
<head>
<style>
body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
.container { max-width: 600px; margin: 0 auto; padding: 20px; }
.header { background-color: #4A90E2; color: white; padding: 20px; text-align: center; border-radius: 5px 5px 0 0; }
.content { background-color: #f9f9f9; padding: 30px; border: 1px solid #ddd; }
.instructions { background-color: white; padding: 20px; margin: 20px 0; border-left: 4px solid #4A90E2; }
.footer { text-align: center; padding: 20px; color: #666; font-size: 0.9em; }
ol { padding-left: 20px; }
li { margin: 10px 0; }
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1>Thank You for Your Purchase!</h1>
  </div>
  <div class="content">
    <p>Thank you for purchasing BreachLine Premium!</p>
    
    <p>Your license is attached to this email. This license is valid for <strong>%d days</strong>, expiring on <strong>%s</strong>.</p>
    
    <div class="instructions">
      <h2>How to Import Your License</h2>
      <ol>
        <li>Open BreachLine on your computer</li>
        <li>Go to <strong>File → Import License File</strong></li>
        <li>Select the license file attached to this email (<code>breachline-license.lic</code>)</li>
      </ol>
      <p>Your Premium features will be activated immediately!</p>
    </div>
    
    <p>If you have any questions or need assistance, please don't hesitate to contact us.</p>
    
    <p>Thank you for choosing BreachLine!</p>
    
    <p><strong>Best regards,</strong><br>
    The BreachLine Team</p>
  </div>
  <div class="footer">
    <p>This email was sent to %s</p>
  </div>
</div>
</body>
</html>
`, msg.ValidDays, msg.ExpirationDate, msg.Email)

	// Create the raw email message with attachment
	logger.Info("Creating email with license attachment")
	rawMessage := createRawEmailWithAttachment(
		sesSenderEmail,
		msg.Email,
		subject,
		bodyText,
		bodyHTML,
		licenseBytes,
		"breachline-license.lic",
	)
	logger.Info("Email message created",
		"size_bytes", len(rawMessage))

	// Send the email using SES
	logger.Info("Sending email via AWS SES",
		"from", sesSenderEmail,
		"to", msg.Email)

	input := &ses.SendRawEmailInput{
		Source: &sesSenderEmail,
		Destinations: []string{
			msg.Email,
		},
		RawMessage: &types.RawMessage{
			Data: []byte(rawMessage),
		},
	}

	result, err := sesClient.SendRawEmail(ctx, input)
	if err != nil {
		logger.Error("Failed to send email via SES",
			"error", err,
			"email", msg.Email,
			"from", sesSenderEmail,
			"possible_causes", []string{
				"unverified_sender",
				"ses_sandbox_mode",
				"insufficient_permissions",
				"rate_limits_exceeded",
			})
		return fmt.Errorf("failed to send email via SES: %w", err)
	}

	logger.Info("Email sent successfully",
		"recipient", msg.Email,
		"ses_message_id", *result.MessageId)
	return nil
}

// createRawEmailWithAttachment creates a MIME multipart email with an attachment
func createRawEmailWithAttachment(from, to, subject, bodyText, bodyHTML string, attachment []byte, filename string) string {
	boundary := "----=_Part_0_123456789.123456789"

	var email string

	// Email headers
	email += fmt.Sprintf("From: %s\r\n", from)
	email += fmt.Sprintf("To: %s\r\n", to)
	email += fmt.Sprintf("Subject: %s\r\n", subject)
	email += "MIME-Version: 1.0\r\n"
	email += fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary)
	email += "\r\n"

	// Text and HTML parts (multipart/alternative)
	email += fmt.Sprintf("--%s\r\n", boundary)
	email += "Content-Type: multipart/alternative; boundary=\"----=_Part_1_123456789.123456789\"\r\n"
	email += "\r\n"

	// Plain text version
	email += "------=_Part_1_123456789.123456789\r\n"
	email += "Content-Type: text/plain; charset=UTF-8\r\n"
	email += "Content-Transfer-Encoding: 7bit\r\n"
	email += "\r\n"
	email += bodyText
	email += "\r\n"

	// HTML version
	email += "------=_Part_1_123456789.123456789\r\n"
	email += "Content-Type: text/html; charset=UTF-8\r\n"
	email += "Content-Transfer-Encoding: 7bit\r\n"
	email += "\r\n"
	email += bodyHTML
	email += "\r\n"

	email += "------=_Part_1_123456789.123456789--\r\n"

	// Attachment
	email += fmt.Sprintf("--%s\r\n", boundary)
	email += fmt.Sprintf("Content-Type: application/octet-stream; name=\"%s\"\r\n", filename)
	email += "Content-Transfer-Encoding: base64\r\n"
	email += fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", filename)
	email += "\r\n"

	// Encode attachment as base64
	encoded := base64.StdEncoding.EncodeToString(attachment)
	// Split into 76-character lines as per RFC 2045
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		email += encoded[i:end] + "\r\n"
	}

	// End of multipart
	email += fmt.Sprintf("--%s--\r\n", boundary)

	return email
}

func main() {
	lambda.Start(Handler)
}
