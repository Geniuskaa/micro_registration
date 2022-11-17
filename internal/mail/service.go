package mail

import (
	"context"
	"errors"
	"fmt"
	"github.com/Geniuskaa/micro_registration/internal/config"
	"github.com/Geniuskaa/micro_registration/internal/parser"
	"github.com/Geniuskaa/micro_registration/internal/sports/karate"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"go.uber.org/zap"
	"io"
	"regexp"
	"strings"
	"time"
)

var (
	errWithMsgReading    = errors.New("Проблема с чтением одного или нескольких писем")
	errWithDBWriting     = errors.New("Проблема с записью данных в БД")
	errWithIncorrectData = errors.New("Too many mistakes in file.")
	errWithResponseType  = errors.New("Undefined type of response!")
)

const (
	// Регулярное выражение для фильтрации писем по их теме. Если соответствует - программа читает письмо.
	SUBJ_REGEX             = "оревнования\\s[0-9]{2}.[0-9]{2}.[0-9]{4}"
	SUBJ_REGEX_AlTERNATIVE = "Соревнования.+[0-9]{2}.[0-9]{2}.[0-9]{4}"

	// Добавочное ключевое слово к вышенаписанному рег. выражению. При наличии в базе участников 1-го письма, выбранное
	// пользователем содержание нового письма с этим ключевым словом изменит/добавит информацию об участнике соревнований.
	CORRECTIONS_KEY = "изменения"

	// Кол-во возможных изменений данных участников. После исчерпания лимита, сам пользователь уже не сможет внести изменения.
	COUNT_OF_EDITS = 2

	// В случае проблем с подключением к почтовому ящику система попытается выполнить несколько переподключений с
	// опред-ым интервалом. Если попыткы были безуспешны система запишет ошибку в канал.
	COUNT_OF_RECONNECTIONS = 5
	RECONNECT_INTERVAL     = time.Minute * 10

	// Расширение файла, файлы с которым обрабатывает наш парсер.
	FILE_EXTENSION = ".xlsx"
)

type Service struct {
	mailboxes              []connectionCredentials
	countOfmailsPerRequest uint32
	logger                 *zap.Logger
	karateServ             *karate.Service
	ctx                    context.Context
}

type fileParser interface {
	ParseXlsx(r io.Reader) (*parser.Response, error)
}

// Структура закрепленная за своим почтовым ящиком. 1 ящику 1 структура.
// Значение этого map это кол-во оставшихся исправлений.
type connectionCredentials struct {
	hostname      string
	port          string
	username      string
	password      string
	previousMails map[seenLetter]uint8
}

// Структура-ключ. У почтового ящика, хранится инфа (в map) об уже прочитанных письмах.
type seenLetter struct {
	date string
	from string
}

//type SportService interface {
//	SportName() string
//}

func NewService(conf *config.Entity, logger *zap.Logger, karateServ *karate.Service) *Service {
	mailBoxes := make([]connectionCredentials, len(conf.Mail.Hostname))

	for i, _ := range conf.Mail.Hostname {
		mailBoxes[i] = connectionCredentials{
			hostname:      conf.Mail.Hostname[i],
			port:          conf.Mail.Port,
			username:      conf.Mail.Username[i],
			password:      conf.Mail.Password[i],
			previousMails: make(map[seenLetter]uint8, 100),
		}
	}

	return &Service{mailboxes: mailBoxes, countOfmailsPerRequest: conf.Mail.CountOfMails, logger: logger, karateServ: karateServ}
}

// Ф-ци, которая динамически меняет кол-во минимально читаемых писем. Для изменения этого числа нужно в конфиге
// "mailboxes" изменить и сохранить поле "MAIL_COUNT_OF_MAILS".
func (s *Service) ChangeCountOfMailsPerReq(count uint32) {
	s.countOfmailsPerRequest = count
}

// В случае пяти подряд неуспешных подключений к почтовому ящику. Горутина запишет ошибку в канал.
func (s *Service) CheckMails() chan error {
	errChn := make(chan error, len(s.mailboxes))

	for _, mailBox := range s.mailboxes {
		go func(errChn chan error, connData connectionCredentials, logger *zap.Logger) {
			var err error
			for i := 0; i < COUNT_OF_RECONNECTIONS; i++ {
				err = s.readLetters(connData, parser.Impl{})
				if err == nil {
					i = 0
					time.Sleep(time.Hour)
					continue
				} else if errors.Is(err, errWithMsgReading) || errors.Is(err, errWithDBWriting) {
					break
				}

				logger.Warn("readLetters returned err. We are trying to reconnect", zap.String("mail-box: ", connData.username))
				time.Sleep(RECONNECT_INTERVAL)
			}

			logger.Error("Unfortunately, we were unable to read mails", zap.String("mail-box: ", connData.username), zap.Error(err))
			errChn <- err

		}(errChn, mailBox, s.logger)
	}

	return errChn
}

func (s *Service) readLetters(conn connectionCredentials, parser fileParser) error {

	// Connect to server
	c, err := client.DialTLS(fmt.Sprintf("%s:%s", conn.hostname, conn.port), nil)
	if err != nil {
		return fmt.Errorf("client.DialTLS failed: %w", err)
	}

	defer c.Logout()

	// Login
	if err := c.Login(conn.username, conn.password); err != nil {
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
	// P.S: Если у нас в самом верху прочитанные (больше минимального кол-ва читаемых сообщений за раз) письма,
	// а ниже идут непрочитанные, то система их не прочтет, т.к ф-ция пробегается только от верха почтового ящика и
	// читает до тех пор пока не встретит хоть одно прочитанное сообщение. При нормальной работе приложения это
	// не должно стать проблемой, но иметь в виду стоит.
	newCount := minCountOfUnReadMsg(c, mbox, s.countOfmailsPerRequest)

	// Get the last n = "countOfmailsPerRequest" messages
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

	countOfMailWithErrs := 0

	auth := s.mailboxAuth(conn)

	for msg := range messages {

		r := msg.GetBody(&section)
		if r == nil {
			s.logger.Error("Server didn't returned message body", zap.String("source: ", "msg.GetBody"))
			countOfMailWithErrs++
			continue
		}

		mr, err := mail.CreateReader(r)
		if err != nil {
			s.logger.Error("Mail reader creation err", zap.Error(fmt.Errorf("mail.CreateReader failed: %w", err)))
			countOfMailWithErrs++
			continue
		}

		// Contains some info about the message
		header := mr.Header

		dateOfMsg, err := header.Date()
		if err != nil {
			s.logger.Error("Header date getting err", zap.Error(fmt.Errorf("header.Date failed: %w", err)))
			countOfMailWithErrs++
			continue
		}

		from, err := header.AddressList("From")
		if err != nil {
			s.logger.Error("Header sender address getting err", zap.Error(fmt.Errorf("header.AddressList failed: %w", err)))
			countOfMailWithErrs++
			continue
		}

		to, err := header.AddressList("To")
		if err != nil {
			s.logger.Error("Header receiver address getting err", zap.Error(fmt.Errorf("header.AddressList failed: %w", err)))
			countOfMailWithErrs++
			continue
		}
		f := regexp.MustCompile("[a-z0-9.]+@[a-z.]+").FindString(from[0].Address)
		t := regexp.MustCompile("[a-z0-9.]+@[a-z.]+").FindString(to[0].Address)

		subject, err := header.Subject()
		if err != nil {
			s.logger.Error("Header subject getting err", zap.Error(fmt.Errorf("header.Subject failed: %w", err)))
			countOfMailWithErrs++
			continue
		}

		matched, err := regexp.MatchString(SUBJ_REGEX, subject)
		if err != nil || !matched {
			s.logger.Info("Letter`s subject is unmatch regex", zap.String("source", "readLetters"),
				zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
					dateOfMsg.Format("02-01-2006"), f, t)))
			countOfMailWithErrs++
			continue
		}

		// Вторым аргументов идет дата, если не так написана - кидаем ошибку
		compDateStr := strings.Split(subject, " ")

		isItDate, err := regexp.MatchString("[0-9]{2}.[0-9]{2}.[0-9]{4}", compDateStr[1])
		if !isItDate {
			s.logger.Warn("Second argument of letter subject is not date or it has written incorrect",
				zap.String("source", "readLetters"), zap.String("letter-info",
					fmt.Sprintf("msg sent: %s from: %s to: %s",
						dateOfMsg.Format("02-01-2006"), f, t)))
			countOfMailWithErrs++
			continue
		}

		compDate, err := time.Parse("02.01.2006", compDateStr[1])
		if err != nil {
			s.logger.Warn("Subject date parsing err", zap.Error(fmt.Errorf("readLetters failed: %w", err)),
				zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
					dateOfMsg.Format("02-01-2006"), f, t)))
			countOfMailWithErrs++
			continue
		}

		if time.Now().After(compDate) {
			s.logger.Warn("Subject date is non actual", zap.String("letter-info",
				fmt.Sprintf("msg sent: %s from: %s to: %s",
					dateOfMsg.Format("02-01-2006"), f, t)))
			countOfMailWithErrs++
			continue
		}

		//TODO: проверка есть ли вообще соревнование в такую дату? Если есть идем дальше, если нет то пропускаем сообщение
		// проверяем в REDIS
		// date.Format("02-01-2006")

		newLetter := seenLetter{
			date: compDate.Format("02-01-2006"),
			from: f,
		}

		restOfCorrections, found := conn.previousMails[newLetter]

		// Если мы уже читали письмо с такими атрибутами и у этого владельца исчерпан лимит исправлений
		// (по-умолчанию он = COUNT_OF_EDITS), то в 3-ий и более раз мы даже не будем открывать это письмо
		// Если же владелец письма в теме указал 'изменения' и у него есть лимит, необходимого внести изменения в базу
		// UPD: Для тех участников (полей) которые помечены ключевым словом см. в "CORRECTIONS_KEY"
		if found && restOfCorrections <= 0 {
			countOfMailWithErrs++
			continue
		} else if found && restOfCorrections > 0 && !strings.Contains(strings.ToLower(subject), CORRECTIONS_KEY) {
			countOfMailWithErrs++
			continue
		}

		// Если вышестоящие фильтры пройдены, смотрим на расширение файла и отправляем его в парсер.
		i := 1
		for {
			//Это значит, что наш парсер не будет обрабатывать больше чем 1 файл в письме.
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
				countOfMailWithErrs++
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
					countOfMailWithErrs++
					continue
				}
				if !strings.HasSuffix(filename, FILE_EXTENSION) {
					s.logger.Error("Filename has wrong file extension", zap.String("letter-info",
						fmt.Sprintf("msg sent: %s from: %s to: %s",
							dateOfMsg.Format("02-01-2006"), f, t)))
					countOfMailWithErrs++
					continue
				}

				i--

				// Ф-ция пока без использования горутин. Позже если появится вариант лучше - заменим.
				response, err := parser.ParseXlsx(p.Body)
				if err != nil {
					s.logger.Error("Err during parsing file", zap.Error(fmt.Errorf("ParseXlsx failed: %w", err)))

					// TODO: отправить пользователю информацию о том, что его файл некорректный
					errOfResp := s.responseToLetter(f, subject, conn, auth, serviceResponseDTO{Err: err})
					if errOfResp != nil {
						// TODO: обработать
					}
					countOfMailWithErrs++
					continue
				}

				if response.PercentErrs > 50 {
					s.logger.Error("Too much errors during file parsing",
						zap.Error(fmt.Errorf("ParseXlsx failed: %w", errWithIncorrectData)))
					// TODO: отправить пользователю информацию о том, что его файл не корректный
					//  возвращать кастомный вариант ошибки
					errOfResp := s.responseToLetter(f, subject, conn, auth, serviceResponseDTO{Err: err})
					if errOfResp != nil {
						// TODO: обработать
					}
					countOfMailWithErrs++
					continue

					// TODO: в таком случае нам лучше не добавлять участников из такого файла, а отправить ответное
					//  письмо с просьбой изменить данные на корректные
				}

				switch response.SportType {
				case "KARATE":
					karateResp, err := s.karateServ.UploadParticipants(response.Map, response.UUID)
					if err != nil {
						s.logger.Error("s.karateServ.UploadParticipants failed: ", zap.Error(err))
						return errWithDBWriting
					}

					errOfResp := s.responseToLetter(f, subject, conn, auth, servResponseToDTOConverter(*karateResp))
					if errOfResp != nil {
						fmt.Println("Ошибка отправки письма")
						//TODO: обработать
					}
				default:

				}

				if err == nil {
					if !found {
						conn.previousMails[newLetter] = COUNT_OF_EDITS
					} else {
						conn.previousMails[newLetter]--
					}
				} else {
					s.logger.Error("Xlsx file parsing err", zap.Error(fmt.Errorf("parser.ParseXlsx failed: %w", err)),
						zap.String("letter-info", fmt.Sprintf("msg sent: %s from: %s to: %s",
							dateOfMsg.Format("02-01-2006"), f, t)))
				}
			}
		}
	}

	//// Повнимательнее с этим местом, как бы это не стало узким горлышком....
	//if countOfMailWithErrs > len(messages)/2 {
	//	return fmt.Errorf("countOfErrs more than half of all msgs: %w", errWithMsgReading)
	//}

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

func servResponseToDTOConverter(resp interface{}) serviceResponseDTO {

	switch resp.(type) {
	case karate.Response:
		karResp := resp.(karate.Response)

		return serviceResponseDTO{
			Err:               nil,
			CountOfFailedRows: karResp.CountOfFailedRows,
			ErrsOfFailedRows:  karResp.ErrsOfFailedRows,
			AddedParticipants: karResp.AddedParticipants,
			CountOfAddedParts: karResp.CountOfAddedParts,
		}
	default:
		return serviceResponseDTO{Err: fmt.Errorf("servResponseToDTOConverter failed: %w", errWithResponseType)}
	}
}
