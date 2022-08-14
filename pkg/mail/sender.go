package mail

import (
	"bytes"
	"fmt"
	"go.uber.org/zap"
	"html/template"
	"net/smtp"
	"strings"
)

const (
	SMTP_PORT = 587
)

type serviceResponseDTO struct {
	Err               error
	CountOfFailedRows int
	ErrsOfFailedRows  []error
	AddedParticipants []string
	CountOfAddedParts int
}

func parseTemplate(subject string, data interface{}, templateFileName ...string) ([]byte, error) {

	t, err := template.ParseFiles(templateFileName...)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	buf.Write([]byte(fmt.Sprintf("Subject: %s \n%s\n\n", subject, mimeHeaders)))

	if err = t.Execute(buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *Service) mailboxAuth(mailboxData connectionCredentials) *smtp.Auth {

	user := mailboxData.username
	password := mailboxData.password

	host := strings.ReplaceAll(mailboxData.hostname, "imap", "smtp")

	auth := smtp.PlainAuth("", user, password, host)

	return &auth
}

func (s *Service) responseToLetter(to string, subject string, mailboxData connectionCredentials, auth *smtp.Auth, resp serviceResponseDTO) error {

	addr := fmt.Sprintf("%s:%d", strings.ReplaceAll(mailboxData.hostname, "imap", "smtp"), SMTP_PORT)
	t := []string{to}

	var err error
	var body []byte
	if resp.Err != nil {
		body, err = parseTemplate(subject, nil, "pkg/mail/templates/negativeFeedback.html")
	} else {
		body, err = parseTemplate(subject, resp, "pkg/mail/templates/positiveFeedback.html")
	}
	if err != nil {
		s.logger.Error("responseToLetter failed", zap.Error(err))
		return err
	}

	err = smtp.SendMail(addr, *auth, mailboxData.username, t, body)

	if err != nil {
		s.logger.Error("responseToLetter failed", zap.Error(err))
		return err
	}

	fmt.Println("По идее письмо должно было отправиться....")
	return nil
}

//func sendInstruction() error {
//
//}
