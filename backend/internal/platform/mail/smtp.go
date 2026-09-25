package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/wangn-tech/campus-hub/internal/config"
	"net"
	"net/smtp"
	"strconv"
)

type Sender interface {
	Send(context.Context, string, string, string) error
}
type SMTP struct{ cfg config.MailConfig }

func New(cfg config.MailConfig) *SMTP { return &SMTP{cfg: cfg} }
func (s *SMTP) Send(ctx context.Context, to, subject, body string) error {
	_ = ctx
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Quit()
	if s.cfg.TLSMode == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp server does not support STARTTLS")
		}
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.cfg.Username != "" {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(s.cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	defer w.Close()
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "To: %s\r\nFrom: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", to, s.cfg.From, subject, body)
	_, err = w.Write(msg.Bytes())
	return err
}
