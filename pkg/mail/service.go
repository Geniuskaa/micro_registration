package mail

import (
	"context"
	"errors"
	"fmt"
	"github.com/Geniuskaa/micro_registration/pkg/config"
	"github.com/Geniuskaa/micro_registration/pkg/parser"
	"github.com/Geniuskaa/micro_registration/pkg/sports/karate"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"go.uber.org/zap"
	"io"
	"regexp"
	"strings"
	"time"
)

var errWithMsgReading = errors.New("Проблема с чтением одного или нескольких писем")
var errWithDBWriting = errors.New("Проблема с записью данных в БД")

const (
	SUBJ_REGEX             = "оревнования\\s[0-9]{2}.[0-9]{2}.[0-9]{4}"
	SUBJ_REGEX_AlTERNATIVE = "Соревнования.+[0-9]{2}.[0-9]{2}.[0-9]{4}"
	CORRECTIONS_KEY        = "изменения"
	COUNT_OF_EDITS         = 2
	COUNT_OF_RECONNECTIONS = 5
	RECONNECT_INTERVAL     = time.Minute * 10
)

type Service struct {
	mailboxes              []*connectionCredentials
	countOfmailsPerRequest uint32
	logger                 *zap.Logger
	karateServ             *karate.Service
	ctx                    context.Context
}

type fileParser interface {
	ParseXlsx(r io.Reader) (*parser.Response, error)
}

type connectionCredentials struct {
	hostname      string
	port          string
	username      string
	password      string
	previousMails map[seenLetter]uint8
}

type seenLetter struct {
	date string
	from string
}

func NewService(conf *config.Entity, logger *zap.Logger, karateServ *karate.Service) *Service {
	mailBoxes := make([]*connectionCredentials, len(conf.Mail.Hostname))

	for i, _ := range conf.Mail.Hostname {
		mailBoxes[i] = &connectionCredentials{
			hostname:      conf.Mail.Hostname[i],
			port:          conf.Mail.Port,
			username:      conf.Mail.Username[i],
			password:      conf.Mail.Password[i],
			previousMails: make(map[seenLetter]uint8, 100),
		}
	}

	return &Service{mailboxes: mailBoxes, countOfmailsPerRequest: conf.Mail.CountOfMails, logger: logger, karateServ: karateServ}
}

func (s *Service) ChangeCountOfMailsPerReq(count uint32) {
	s.countOfmailsPerRequest = count
}

// TODO:  В случае пяти подряд неуспешных подключений падает
func (s *Service) CheckMails() chan error {
	errChn := make(chan error, len(s.mailboxes))

	for i, mailBox := range s.mailboxes {
		go func(errChn chan error, connData *connectionCredentials, count uint32, logger *zap.Logger, i int) {
			var err error
			for i := 0; i < COUNT_OF_RECONNECTIONS; i++ {
				err = s.readLetters(connData.hostname, connData.port, connData.username, connData.password, count,
					connData.previousMails, parser.Impl{})
				if err == nil {
					return
				} else if errors.Is(err, errWithMsgReading) || errors.Is(err, errWithDBWriting) {
					break
				}

				logger.Warn("readLetters returned err. We are trying to reconnect", zap.String("mail-box: ", connData.username))
				time.Sleep(RECONNECT_INTERVAL)
			}

			logger.Error("Unfortunately, we were unable to read mails", zap.String("mail-box: ", connData.username), zap.Error(err))
			errChn <- err

		}(errChn, mailBox, s.countOfmailsPerRequest, s.logger, i)
	}

	return errChn
}

func (s *Service) readLetters(hostname string, port string, username string, password string,
	countOfmailsPerRequest uint32, previousMails map[seenLetter]uint8, parser fileParser) error {

	// Connect to server
	c, err := client.DialTLS(fmt.Sprintf("%s:%s", hostname, port), nil)
	if err != nil {
		return fmt.Errorf("client.DialTLS failed: %w", err)
	}

	defer c.Logout()

	// Login
	if err := c.Login(username, password); err != nil {
		return fmt.Errorf("c.Login failed: %w", err)
	}

	// Select INBOX (входящие)
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		return fmt.Errorf("c.Select failed: %w", err)
	}

	// Получение флагов, которые может возвращать почта
	//logger.Info("", zap.Strings("Flags for INBOX:", mbox.Flags))

	// Проверка на то, сколько сообщений нужно прочесть, чтобы не осталось непрочитанных сообщений
	newCount := minCountOfUnReadMsg(c, mbox, countOfmailsPerRequest) // s.countOfmailsPerRequest

	// Get the last n (countOfmailsPerRequest) messages
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > (newCount - 1) {
		// We're using unsigned integers here, only subtract if the result is > 0
		from = mbox.Messages - (newCount - 1)
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	var section imap.BodySectionName
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, newCount+5)
	done := make(chan error, 1)

	done <- c.Fetch(seqset, items, messages)

	countOfErrs := 0

	for msg := range messages {

		r := msg.GetBody(&section)
		if r == nil {
			s.logger.Error("Server didn't returned message body", zap.String("source: ", "msg.GetBody"))
			countOfErrs++
			continue
		}

		mr, err := mail.CreateReader(r)
		if err != nil {
			s.logger.Error("Mail reader creation err", zap.Error(fmt.Errorf("mail.CreateReader failed: %w", err)))
			countOfErrs++
			continue
		}

		// Print some info about the message
		header := mr.Header

		dateOfMsg, err := header.Date()
		if err != nil {
			s.logger.Error("header date getting err", zap.Error(fmt.Errorf("header.Date failed: %w", err)))
			countOfErrs++
			continue
		}

		from, err := header.AddressList("From")
		if err != nil {
			s.logger.Error("Header sender address getting err", zap.Error(fmt.Errorf("header.AddressList failed: %w", err)))
			countOfErrs++
			continue
		}

		to, err := header.AddressList("To")
		if err != nil {
			s.logger.Error("Header reciever address getting err", zap.Error(fmt.Errorf("header.AddressList failed: %w", err)))
			countOfErrs++
			continue
		}
		t := regexp.MustCompile("[a-z0-9.]+@[a-z.]+").FindString(to[0].Address)
		f := regexp.MustCompile("[a-z0-9.]+@[a-z.]+").FindString(from[0].Address)

		subject, err := header.Subject()
		if err != nil {
			s.logger.Error("Header subject getting err", zap.Error(fmt.Errorf("header.Subject failed: %w", err)))
			countOfErrs++
			continue
		}

		matched, err := regexp.MatchString(SUBJ_REGEX, subject)
		if err != nil {
			s.logger.Info("Letter`s subject is unmatch regex", zap.String("source", "readLetters"),
				zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
					dateOfMsg.Format("02-01-2006"), f, t)))
			continue
		}

		if !matched {
			s.logger.Warn("Letter`s subject is unmatch regex", zap.String("source", "readLetters"),
				zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
					dateOfMsg.Format("02-01-2006"), f, t)))
			continue
		}

		// Вторым аргументов идет дата, если не так то ошибка
		compDateStr := strings.Split(subject, " ")

		isItDate, err := regexp.MatchString("[0-9]{2}.[0-9]{2}.[0-9]{4}", compDateStr[1])
		if !isItDate {
			s.logger.Warn("Second argument of letter subject is not date", zap.String("source", "readLetters"),
				zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
					dateOfMsg.Format("02-01-2006"), f, t)))
			continue
		}

		compDate, err := time.Parse("02.01.2006", compDateStr[1])
		if err != nil {
			s.logger.Warn("Subject date parsing err", zap.Error(fmt.Errorf("readLetters failed: %w", err)),
				zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
					dateOfMsg.Format("02-01-2006"), f, t)))
			continue
		}

		if time.Now().After(compDate) {
			s.logger.Warn("Subject date is non actual", zap.String("letter-info",
				fmt.Sprintf("msg sent: %s from: %s to: %s",
					dateOfMsg.Format("02-01-2006"), f, t)))
			continue
		}

		//TODO: проверка есть ли вообще соревнование в такую дату? Если есть идем дальше, если нет то пропускаем сообщение
		// date.Format("02-01-2006")

		newLetter := seenLetter{
			date: dateOfMsg.Format("02-01-2006"),
			from: f,
		}
		fmt.Println("Ключ для проверки письма: ", newLetter)

		restOfCorrections, found := previousMails[newLetter]
		// Если мы уже читали письмо с такими атрибутами и у этого владельца исчерпан лимит исправлений (по-умолчанию он = 2)
		// то в 3-ий и более раз мы даже не будем открывать это письмо
		// Если же владелец письма в теме указал 'изменения' и у него есть лимит, необходимого внести изменения в базу
		if found && restOfCorrections <= 0 {
			continue
		} else if found && restOfCorrections > 0 && !strings.Contains(strings.ToLower(subject), CORRECTIONS_KEY) {
			continue
		}

		// Код для чтения адреса получателя, пока пусть будет. На запас.
		//if to, err := header.AddressList("To"); err == nil {
		//	log.Println(regexp.MustCompile(fmt.Sprintf("From: %s", regexp.MustCompile("[a-z0-9.]+@[a-z.]+").FindString(to[0].Address)))
		//}

		//TODO: если вышестоящие фильтры пройдены, смотрим на расширение файла и отправляем его в парсер.
		// Возможно парсер стоит запускать в горутине, где именно это стоит сделать необходимо подумать.
		i := 1
		for {
			//Это значит, что наш парсер не будет обрабатывать больше чем 1 файл в письме
			if i == 0 {
				break
			}
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				s.logger.Error("Error getting *mail.Part", zap.Error(fmt.Errorf("mr.NextPart failed: %w", err)),
					zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
						dateOfMsg.Format("02-01-2006"), f, t)))
				break
			}

			switch h := p.Header.(type) {
			case *mail.AttachmentHeader:
				// This is an attachment
				filename, err := h.Filename()
				if err != nil {
					s.logger.Error("Err filename getting", zap.Error(fmt.Errorf("h.Filename failed: %w", err)),
						zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
							dateOfMsg.Format("02-01-2006"), f, t)))
					continue
				}
				if !strings.HasSuffix(filename, ".xlsx") {
					continue
				}

				i--

				response, err := parser.ParseXlsx(p.Body) // пока без горутин и с добавлением в базу. Позже если появится вариант лучше - заменим
				if err != nil {

				}

				if response.PercentErrs > 50 {
					err = errors.New("Too many mistakes in file.")
				}

				//TODO: прокинуть данную мапу в слой, который загрузит это в бд
				switch response.SportType {
				case "KARATE":
					err = s.karateServ.UploadParticipants(response.Map, response.UUID) // Реализовать метод
					if err != nil {
						return errWithDBWriting
					}
					for _, v := range response.Map {
						fmt.Println(v.(karate.Participant))
					}
				default:

				}
				fmt.Println("Percent of failed rows is: ", response.PercentErrs, "%")
				fmt.Println(response.UUID) //TODO: Удалить это нужно потом

				if err == nil {
					if !found {
						previousMails[newLetter] = COUNT_OF_EDITS
					} else {
						previousMails[newLetter]--
					}
				} else {
					s.logger.Error("Xlsx file parsing err", zap.Error(fmt.Errorf("parser.ParseXlsx failed: %w", err)),
						zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
							dateOfMsg.Format("02-01-2006"), f, t)))
				}
			}
		}
	}

	if countOfErrs != 0 {
		return fmt.Errorf("countOfErrs more than 0: %w", errWithMsgReading)
	}

	if err := <-done; err != nil {
		return fmt.Errorf("done chan returned err: %w", errWithMsgReading)
	}

	return nil
}

func minCountOfUnReadMsg(c *client.Client, mbox *imap.MailboxStatus, baseCount uint32) uint32 {
	i := 1
	for {
		from := uint32(1)
		to := mbox.Messages
		if mbox.Messages > (baseCount - 1) {
			// We're using unsigned integers here, only subtract if the result is > 0
			from = mbox.Messages - (baseCount - 1)
		}
		seqset := new(imap.SeqSet)
		seqset.AddRange(from, to)

		messages := make(chan *imap.Message, baseCount+5)
		done := make(chan error, 1)

		// Читает письма до тех пор пока не упрется в ошибку (=конец интервала сообщений)
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchFlags}, messages)

		msg := <-messages
		if msg.Flags == nil || len(msg.Flags) == 0 || !strings.EqualFold(msg.Flags[0], "\\Seen") {
			i *= 2
			baseCount += uint32(i)
			continue
		}

		return baseCount
	}
}
