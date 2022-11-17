package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"strings"
)

func main() {

	k := make([]int, 0, 12)
	k = append(k, 3, 6, 2)
	fmt.Println(len(k))
	fmt.Println(cap(k))
	//responseToLetter()
}

type mailBuilder struct {
	Sender  string
	To      []string
	Subject string
	Body    string //serviceResponseDTO
}

type serviceResponseDTO struct {
	Err               error
	CountOfFailedRows int
	ErrsOfFailedRows  []error
	AddedParticipants []string
	CountOfAddedParts int
}

func responseToLetter() error {

	user := "rafishir@mail.ru"
	password := "fi6QMx52K2G9y0qYAbd3"

	host := "smtp.mail.ru"

	authP := smtp.PlainAuth("", user, password, host)

	//addr := fmt.Sprintf("%s:%d", strings.ReplaceAll(mailboxData.hostname, "imap", "smtp"), SMTP_PORT)
	t := []string{"eminizinar@gmail.com"}

	tmpl, err := template.ParseFiles("C:\\code\\GoLang\\micro_registration\\pkg\\mail\\templates\\positiveFeedback.html")
	if err != nil {
		log.Println(err)
	}
	buf := new(bytes.Buffer)
	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	buf.Write([]byte(fmt.Sprintf("Subject: Соревнования 26.09.2022 \n%s\n\n", mimeHeaders)))

	data := serviceResponseDTO{
		Err:               nil,
		CountOfFailedRows: 2,
		ErrsOfFailedRows:  nil,
		AddedParticipants: []string{"Машка Кукарова", "Петя Затаков", "Кеша Иванов", "Маня Улетов"},
		CountOfAddedParts: 4,
	}

	if err = tmpl.Execute(buf, data); err != nil {
		log.Println(err)
	}

	//m := mailBuilder{
	//	Sender:  user,
	//	To:      t,
	//	Subject: "test mail",
	//	Body:    body,
	//}
	//
	//msg := buildMessage(m)

	err = smtp.SendMail(host+":587", authP, user, t, buf.Bytes()) // port = 587

	if err != nil {
		fmt.Println("responseToLetter failed", err)
		return err
	}

	fmt.Println("По идее письмо должно было отправиться....")
	return nil
}

func buildMessage(mail mailBuilder) string {
	msg := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n"
	msg += fmt.Sprintf("From: %s\r\n", mail.Sender)
	msg += fmt.Sprintf("To: %s\r\n", strings.Join(mail.To, ";"))
	msg += fmt.Sprintf("Subject: %s\r\n", mail.Subject)
	msg += fmt.Sprintf("\r\n%s\r\n", mail.Body)

	return msg

	//if mail.Body.err != nil {
	//	msg += fmt.Sprintf("\r\nSome error during work with your file: %w\r\n", mail.Body.err)
	//
	//}
	//msg += fmt.Sprintf("\r\nWe successfully parsed your file: %s\r\n", mail.Body.addedParticipants)

}
