package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

type Mailer struct {
	Host, Port, Username, Password, From, AppURL string
	AllowInsecure                                bool // Only for a local SMTP capture service.
}

func (m Mailer) Configured() bool {
	u, err := url.Parse(m.AppURL)
	from, e := mail.ParseAddress(m.From)
	return m.Host != "" && m.Port != "" && err == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http") && e == nil && from.Address == m.From
}
func (m Mailer) SendVerification(ctx context.Context, email, token string) error {
	return m.sendLink(ctx, email, "Подтвердите email в Taverna", "verify", token, "Подтвердите email, открыв ссылку:")
}
func (m Mailer) SendPasswordReset(ctx context.Context, email, token string) error {
	return m.sendLink(ctx, email, "Восстановление пароля Taverna", "reset", token, "Чтобы задать новый пароль, откройте ссылку:")
}
func (m Mailer) sendLink(ctx context.Context, email, subject, parameter, token, introduction string) error {
	if !m.Configured() {
		return ErrEmailDeliveryUnavailable
	}
	link, _ := url.Parse(m.AppURL)
	link.Fragment = url.Values{parameter: []string{token}}.Encode()
	message := strings.Join([]string{"From: " + m.From, "To: " + email, "Subject: " + mime.QEncoding.Encode("utf-8", subject), "MIME-Version: 1.0", "Content-Type: text/plain; charset=UTF-8", "", "Taverna", introduction, link.String(), "", "Если вы не запрашивали это действие, проигнорируйте письмо."}, "\r\n")
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(m.Host, m.Port))
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err = conn.SetDeadline(deadline); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if !m.AllowInsecure {
		return fmt.Errorf("SMTP requires STARTTLS")
	}
	if m.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", m.Username, m.Password, m.Host)); err != nil {
			return err
		}
	}
	if err = client.Mail(m.From); err != nil {
		return err
	}
	if err = client.Rcpt(email); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = writer.Write([]byte(message)); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
