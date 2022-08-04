package parser

import (
	"errors"
	"fmt"
	"github.com/Geniuskaa/micro_registration/pkg/sports/karate"
	"github.com/xuri/excelize/v2"
	"io"
	"strconv"
	"strings"
)

const (
	SHEET_NAME             = "Лист 1"
	KARATE                 = "KARATE"
	KARATE_KATA            = "кат"
	KARATE_KUMITE          = "кум"
	COUNT_OF_METAINFO_ROWS = 6
)

type Impl struct {
}

//TODO: парсер будет обрабатывать структуры и закинет все данные в мапу с ключом категория (пол+вес+возраст)
// затем полученную мапу мы должны передать в валидатор, после получения одобрения от него отправим мапу в репозиторий
// По UID парсер должен понять какой это вид спорта и использовать соотвествующий парсер
func (i Impl) ParseXlsx(r io.Reader) (int, int, map[string]interface{}, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return -1, 0, nil, fmt.Errorf("excelize.OpenReader failed: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			//x.logger.Panic("", zap.Error(err)) что то с этим сделать
		}
	}()

	rows, err := f.GetRows(SHEET_NAME)
	if err != nil {
		return -1, 0, nil, fmt.Errorf("f.GetRows failed: %w", err)
	}

	// Применять ниже описанную фильтрацию для файлов с кол-вом строк более 40
	// Нужно делать замеры, если процент пустых позиций выше 20% то мы отбросим файл.
	//if len(rows) > 40 {
	//	if percent > 20 {
	//		return
	//	}
	//}

	if len(rows) > 1500 {
		return -1, 0, nil, fmt.Errorf("Xlsx doc has more than 1500 rows. It seems that someone try to DDOS us...")
	}

	//TODO: Сделать аналитику по длине строки. Если, к примеру, более 15. То это спаммер и закончить парсинг

	// По UID мы найдем соревнование в БД
	uid := rows[1][0]
	uidVal, err := strconv.Atoi(uid)
	if err != nil {
		return -1, 0, nil, errors.New("Err uid converting to int")
	}
	// TODO: обращение в REDIS
	sportType := KARATE

	var (
		m           map[string]interface{}
		countOfErrs int
	)
	switch sportType {
	case KARATE:
		countOfErrs, m, err = karateParser(rows)
		if err != nil {
			return -1, 0, nil, fmt.Errorf("karateParser failed: %w", err)
		}
	default:
		return -1, 0, nil, errors.New("We coudln`t determine sport type.")
	}

	fmt.Println(uid)

	return countOfErrs, uidVal, m, nil
}

func karateParser(arr [][]string) (int, map[string]interface{}, error) {
	m := make(map[string]interface{}, len(arr)-2)
	countOfErrs := 0
	countOfEmptyRows := 0

	for i, row := range arr {
		if i < COUNT_OF_METAINFO_ROWS {
			continue
		}

		if row == nil {
			countOfEmptyRows++
			continue
		}

		var (
			err           error
			age, cat, kyi uint8   = 0, 0, 0
			weight        float32 = 0
		)

		katKum := strings.Split(row[5], "/")

		val := strings.ToLower(katKum[0])
		katkumSize := len(katKum)

		doKata, doKumite := false, false

		if katkumSize == 2 || strings.EqualFold(val, KARATE_KUMITE) {
			if katkumSize == 2 && !(strings.EqualFold(strings.ToLower(katKum[0]), KARATE_KATA) &&
				strings.EqualFold(strings.ToLower(katKum[1]), KARATE_KUMITE)) {
				countOfErrs++
				continue
			}
			age, cat, kyi, weight, err = rowKarateConverterKumite(row)
			if err != nil {
				countOfErrs++
				continue
			}
			if katkumSize == 2 {
				doKata = true
				doKumite = true
			} else {
				doKumite = true
			}
		} else if strings.EqualFold(val, KARATE_KATA) {
			age, kyi, err = rowKarateConverterKata(row)
			if err != nil {
				countOfErrs++
				continue
			}
			doKata = true
		} else {
			countOfErrs++
			continue
		}

		entity := karate.Participant{
			FullName:   row[0],
			Sex:        row[1],
			Age:        age,
			Kyi:        kyi,
			City:       row[4],
			KataKumite: [2]bool{doKata, doKumite},
			Weight:     weight,
			Category:   cat,
			Coach:      row[8],
		}

		m[entity.FullName] = entity
	}

	totalParticipants := len(arr) - COUNT_OF_METAINFO_ROWS - countOfEmptyRows
	percentErrs := countOfErrs / totalParticipants * 100

	fmt.Println("Count of empty rows is: ", countOfEmptyRows)

	return percentErrs, m, nil
}

func rowKarateConverterKumite(arr []string) (age, cat, kyi uint8, weight float32, err error) {
	ag, err := strconv.Atoi(arr[2])
	if err != nil {
		return
	}
	age = uint8(ag)

	ky, err := strconv.Atoi(arr[3])
	if err != nil {
		return
	}
	kyi = uint8(ky)

	wei, err := strconv.ParseFloat(arr[6], 32)
	if err != nil {
		return
	}
	weight = float32(wei)

	ca, err := strconv.Atoi(arr[7])
	if err != nil {
		return
	}
	cat = uint8(ca)

	return
}

func rowKarateConverterKata(arr []string) (age, kyi uint8, err error) {
	ag, err := strconv.Atoi(arr[2])
	if err != nil {
		return
	}
	age = uint8(ag)

	ky, err := strconv.Atoi(arr[3])
	if err != nil {
		return
	}
	kyi = uint8(ky)

	return
}
