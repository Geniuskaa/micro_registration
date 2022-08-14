package parser

import (
	"errors"
	"fmt"
	"github.com/Geniuskaa/micro_registration/pkg/sports/karate"
	"github.com/jackc/pgtype"
	"github.com/xuri/excelize/v2"
	"io"
	"strconv"
	"strings"
)

const (
	SHEET_NAME             = "Лист 1"
	COUNT_OF_METAINFO_ROWS = 6

	// Constants for parser protection
	MAX_LEN_OF_ROW                         = 15 // Длина строки измеряется в кол-ве ячеек excel таблицы
	COUNTS_OF_LONG_ROWS_BEFORE_BLOCK_EXCEL = 10

	KARATE_KATA   = "кат"
	KARATE_KUMITE = "кум"

	// SPORT TYPES
	KARATE = "KARATE"
)

type Impl struct {
}

type Response struct {
	SportType   string
	PercentErrs int
	UUID        string
	Map         map[string]interface{}
}

// Парсер обрабатывает структуры и закинет все данные в мапу с ключом "ФИО"
//  затем полученную мапу мы должны передать в валидатор, после получения одобрения от него отправим мапу в репозиторий
//  По UID парсер должен понять какой это вид спорта и использовать соотвествующий парсер
func (i Impl) ParseXlsx(r io.Reader) (*Response, error) { //(sportType string, percentErrs int, uuidVal int, m map[string]interface{}, err error)
	resp := &Response{}

	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("excelize.OpenReader failed: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
		}
	}()

	rows, err := f.GetRows(SHEET_NAME)
	if err != nil {
		return nil, fmt.Errorf("f.GetRows failed: %w", err)
	}

	// Применять ниже описанную фильтрацию для файлов с кол-вом строк более 40
	// Нужно делать замеры, если процент пустых позиций выше 20% то мы отбросим файл.
	//if len(rows) > 40 {
	//	if percent > 20 {
	//		return
	//	}
	//}

	if len(rows) > 1500 {
		return nil, fmt.Errorf("Xlsx doc has more than 1500 rows. It seems that someone try to DDOS us...")
	}

	//TODO: ОБЯЗАТЕЛЬНО!!! Сделать аналитику по длине строки. Если, к примеру, более 15. То это спаммер и закончить парсинг

	// По UID мы найдем соревнование в БД
	resp.UUID = rows[1][0]

	// TODO: обращение в REDIS и возврат вида спорта
	sportType := KARATE

	switch sportType {
	case KARATE:
		percentErrs, m, err := karateParser(rows)
		if err != nil {
			return nil, fmt.Errorf("karateParser failed: %w", err)
		}
		resp.PercentErrs = percentErrs
		resp.Map = m
		resp.SportType = sportType

		return resp, nil
	default:
		return nil, errors.New("We coudln`t determine sport type.")
	}

}

func karateParser(arr [][]string) (percentErrs int, m map[string]interface{}, err error) {
	m = make(map[string]interface{}, len(arr)-2)
	countOfErrs := 0
	countOfEmptyRows := 0
	countOfVeryLongRows := 0

	for i, row := range arr {
		if i < COUNT_OF_METAINFO_ROWS {
			continue
		}

		if row == nil || row[0] == "" || row[1] == "" || row[2] == "" || row[5] == "" {
			countOfEmptyRows++
			continue
		}

		// Если часто (задаётся константно) попадаются длинные строки, система сочтёт это за спам
		if len(row) > MAX_LEN_OF_ROW {
			countOfVeryLongRows++
			if countOfVeryLongRows > COUNTS_OF_LONG_ROWS_BEFORE_BLOCK_EXCEL {
				return countOfErrs, nil, errors.New("We found too much long rows. It seems that it is spam.")
			}
		}

		var (
			err           error
			age, kyi, dan uint8   = 6, 0, 0
			weight        float32 = 11
			kataGroup     bool    = false
			cat           pgtype.Int4range
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
			age, kyi, dan, cat, kataGroup, weight, err = rowKarateConverterKumite(row)
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
			age, kyi, dan, kataGroup, err = rowKarateConverterKata(row)
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
			Dan:        dan,
			City:       row[4],
			KataKumite: [2]bool{doKata, doKumite},
			KataGroup:  kataGroup,
			Weight:     weight,
			Category:   cat,
			Coach:      row[9],
		}

		m[entity.FullName] = entity
	}

	totalParticipants := len(arr) - COUNT_OF_METAINFO_ROWS - countOfEmptyRows
	percentErrs = countOfErrs / totalParticipants * 100

	return percentErrs, m, nil
}

func rowKarateConverterKumite(arr []string) (age, kyi, dan uint8, cat pgtype.Int4range, kataGroup bool, weight float32, err error) {
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

	wei, err := strconv.ParseFloat(arr[7], 32)
	if err != nil {
		return
	}
	weight = float32(wei)

	arg := strings.ToLower(arr[6])
	if strings.EqualFold(arg, "да") {
		kataGroup = true
	} else if strings.EqualFold(arg, "нет") {
		kataGroup = false
	}

	absolute := strings.HasSuffix(arr[8], "+")
	if absolute {
		temp := strings.TrimRight(arr[8], "+")
		err = cat.Set(fmt.Sprintf("[%s,)", temp))
		if err != nil {
			return
		}
	} else {
		upper, err := strconv.Atoi(arr[8])
		if err != nil {
			return 0, 0, 0, pgtype.Int4range{}, false, 0, err
		}
		if age >= 18 {
			err = cat.Set(fmt.Sprintf("[%d,%d)", upper-10, upper+1))

			if err != nil {
				return 0, 0, 0, pgtype.Int4range{}, false, 0, err
			}
		} else {
			err = cat.Set(fmt.Sprintf("[%d,%d)", upper-5, upper+1))

			if err != nil {
				return 0, 0, 0, pgtype.Int4range{}, false, 0, err
			}
		}

	}

	da, err := strconv.Atoi(arr[10])
	if err != nil {
		return
	}
	dan = uint8(da)

	return
}

func rowKarateConverterKata(arr []string) (age, kyi, dan uint8, kataGroup bool, err error) {
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

	arg := strings.ToLower(arr[6])
	if strings.EqualFold(arg, "да") {
		kataGroup = true
	} else if strings.EqualFold(arg, "нет") {
		kataGroup = false
	}

	da, err := strconv.Atoi(arr[10])
	if err != nil {
		return
	}
	dan = uint8(da)

	return
}
