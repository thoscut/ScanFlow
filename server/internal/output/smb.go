package output

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

// SMBHandler uploads documents to a SMB/CIFS network share.
type SMBHandler struct {
	server          string
	share           string
	username        string
	password        string
	directory       string
	filenamePattern string
}

// NewSMBHandler creates a new SMB output handler.
func NewSMBHandler(cfg config.SMBConfig) *SMBHandler {
	password := ""
	if cfg.PasswordFile != "" {
		data, err := os.ReadFile(cfg.PasswordFile)
		if err == nil {
			password = strings.TrimSpace(string(data))
		}
	}

	return &SMBHandler{
		server:          cfg.Server,
		share:           cfg.Share,
		username:        cfg.Username,
		password:        password,
		directory:       cfg.Directory,
		filenamePattern: cfg.FilenamePattern,
	}
}

func (h *SMBHandler) Name() string { return "smb" }

func (h *SMBHandler) Available() bool {
	return h.server != "" && h.share != ""
}

// Send uploads a document to the SMB share.
func (h *SMBHandler) Send(ctx context.Context, doc *jobs.Document) error {
	// Connect to SMB server
	server := h.server
	if !strings.Contains(server, ":") {
		server = server + ":445"
	}
	// Strip any leading "//" from the server address
	server = strings.TrimPrefix(server, "//")
	if !strings.Contains(server, ":") {
		server = server + ":445"
	}

	conn, err := net.DialTimeout("tcp", server, 10*time.Second)
	if err != nil {
		return fmt.Errorf("SMB connect: %w", err)
	}
	defer conn.Close()

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     h.username,
			Password: h.password,
		},
	}

	session, err := d.Dial(conn)
	if err != nil {
		return fmt.Errorf("SMB authenticate: %w", err)
	}
	defer session.Logoff()

	share, err := session.Mount(h.share)
	if err != nil {
		return fmt.Errorf("SMB mount share: %w", err)
	}
	defer share.Umount()

	// Ensure directory exists
	if h.directory != "" {
		share.MkdirAll(h.directory, 0o755)
	}

	// Build filename
	filename := h.buildFilename(doc)
	path := filename
	if h.directory != "" {
		path = h.directory + "/" + filename
	}

	// Write file
	f, err := share.Create(path)
	if err != nil {
		return fmt.Errorf("SMB create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, doc.Reader); err != nil {
		return fmt.Errorf("SMB write: %w", err)
	}

	return nil
}

func (h *SMBHandler) buildFilename(doc *jobs.Document) string {
	if doc.Filename != "" {
		return doc.Filename
	}

	pattern := h.filenamePattern
	if pattern == "" {
		pattern = "{date}_{time}_{title}"
	}

	now := time.Now()
	filename := pattern
	filename = strings.ReplaceAll(filename, "{date}", now.Format("20060102"))
	filename = strings.ReplaceAll(filename, "{time}", now.Format("150405"))

	title := "scan"
	if doc.Title != "" {
		title = doc.Title
	}
	filename = strings.ReplaceAll(filename, "{title}", title)

	if !strings.HasSuffix(filename, ".pdf") {
		filename += ".pdf"
	}

	return filename
}
