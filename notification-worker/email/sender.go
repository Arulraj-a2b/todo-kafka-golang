package email

import (
	"log"
	"os"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// Send fires off a transactional email. Best-effort: logs and swallows errors,
// because the user's todo write has already committed in todo-service.
// No-op when SENDGRID_API_KEY is unset, so the worker stays runnable in dev.
func Send(toEmail, toName, subject, plainText, htmlBody string) {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	if apiKey == "" {
		return
	}

	from := mail.NewEmail(os.Getenv("SENDGRID_FROM_NAME"), os.Getenv("SENDGRID_FROM_EMAIL"))
	to := mail.NewEmail(toName, toEmail)
	msg := mail.NewSingleEmail(from, subject, to, plainText, htmlBody)

	client := sendgrid.NewSendClient(apiKey)
	resp, err := client.Send(msg)
	if err != nil {
		log.Printf("sendgrid: send failed: %v", err)
		return
	}
	if resp.StatusCode >= 300 {
		log.Printf("sendgrid: non-2xx status=%d body=%s", resp.StatusCode, resp.Body)
	}
}
