package output

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"strings"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

// EmailHandler sends documents via email as attachments.
type EmailHandler struct {
	host      string
	port      int
	user      string
	password  string
	from      string
	recipient string
}

// NewEmailHandler creates a new email output handler.
func NewEmailHandler(cfg config.EmailConfig) *EmailHandler {
	password := ""
	if cfg.SMTPPasswordFile != "" {
		data, err := os.ReadFile(cfg.SMTPPasswordFile)
		if err == nil {
			password = strings.TrimSpace(string(data))
		}
	}

	return &EmailHandler{
		host:      cfg.SMTPHost,
		port:      cfg.SMTPPort,
		user:      cfg.SMTPUser,
		password:  password,
		from:      cfg.FromAddress,
		recipient: cfg.DefaultRecipient,
	}
}

func (h *EmailHandler) Name() string { return "email" }

func (h *EmailHandler) Available() bool {
	return h.host != "" && h.from != "" && h.recipient != ""
}

// Send emails a document as an attachment.
func (h *EmailHandler) Send(_ context.Context, doc *jobs.Document) error {
	to := h.recipient
	subject := "ScanFlow: " + doc.Filename
	if doc.Title != "" {
		subject = "ScanFlow: " + doc.Title
	}

	// Read document data
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, doc.Reader); err != nil {
		return fmt.Errorf("read document: %w", err)
	}

	// Build MIME email with attachment
	boundary := "ScanFlow-Boundary-12345"
	var msg bytes.Buffer

	msg.WriteString(fmt.Sprintf("From: %s\r\n", h.from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n", boundary))

	// Text body
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	msg.WriteString(fmt.Sprintf("Scanned document: %s\r\n\r\n", doc.Filename))

	// PDF attachment
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString(fmt.Sprintf("Content-Type: application/pdf; name=\"%s\"\r\n", doc.Filename))
	msg.WriteString("Content-Transfer-Encoding: base64\r\n")
	msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", doc.Filename))

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	// Wrap at 76 characters
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		msg.WriteString(encoded[i:end] + "\r\n")
	}

	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	// Send via SMTP
	addr := fmt.Sprintf("%s:%d", h.host, h.port)
	auth := smtp.PlainAuth("", h.user, h.password, h.host)

	if err := smtp.SendMail(addr, auth, h.from, []string{to}, msg.Bytes()); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}
